package apex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	pollInterval       = 5 * time.Second
	wgConfiPermissions = 0600
	wgBinary           = "wg"
	wgWinBinary        = "wireguard.exe"
	REGISTER_URL       = "/api/zones/%s/peers"
	DEVICE_URL         = "/api/devices/%s"
)

// TODO: A nicely typed REST API client library
// but for now!
type Peer struct {
	ID          string `json:"id"`
	DeviceID    string `json:"device-id"`
	ZoneID      string `json:"zone-id"`
	EndpointIP  string `json:"endpoint-ip"`
	AllowedIPs  string `json:"allowed-ips"`
	NodeAddress string `json:"node-address"`
	ChildPrefix string `json:"child-prefix"`
	HubRouter   bool   `json:"hub-router"`
	HubZone     bool   `json:"hub-zone"`
	ZonePrefix  string `json:"zone-prefix"`
}

type DeviceJSON struct {
	ID        string `json:"id"`
	PublicKey string `json:"public-key"`
	UserID    string `json:"user-id"`
}

type Apex struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPubKeyInConfig bool
	controllerIP            string
	controllerPasswd        string
	listenPort              int
	zone                    string
	requestedIP             string
	userProvidedEndpointIP  string
	localEndpointIP         string
	childPrefix             string
	stun                    bool
	hubRouter               bool
	hubRouterWgIP           string
	os                      string
	wgConfig                wgConfig
	withToken               string
	auth                    Authenticator
	controllerURL           *url.URL
	peerCache               map[string]Peer
	keyCache                map[string]string
	wgLocalAddress          string
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          string
	PersistentKeepAlive string
	// AllowedIPs []string `delim:","` TODO: support an AllowedIPs slice here
}

type wgLocalConfig struct {
	PrivateKey string
	ListenPort int
}

func NewApex(ctx context.Context, cCtx *cli.Context) (*Apex, error) {
	controller := cCtx.Args().First()
	if controller == "" {
		log.Fatal("<controller-url> required")
	}

	// check that it's a valid URL
	controllerURL, err := url.Parse(controller)
	if err != nil {
		log.Fatalf("error checking controller url: %+v", err)
	}

	if err := checkOS(); err != nil {
		return nil, err
	}

	ax := &Apex{
		wireguardPubKey:        cCtx.String("public-key"),
		wireguardPvtKey:        cCtx.String("private-key"),
		controllerIP:           cCtx.String("controller"),
		controllerPasswd:       cCtx.String("controller-password"),
		listenPort:             cCtx.Int("listen-port"),
		requestedIP:            cCtx.String("request-ip"),
		userProvidedEndpointIP: cCtx.String("local-endpoint-ip"),
		childPrefix:            cCtx.String("child-prefix"),
		stun:                   cCtx.Bool("stun"),
		hubRouter:              cCtx.Bool("hub-router"),
		withToken:              cCtx.String("with-token"),
		controllerURL:          controllerURL,
		os:                     GetOS(),
		peerCache:              make(map[string]Peer),
		keyCache:               make(map[string]string),
		wgLocalAddress:         "",
	}

	if ax.os == Windows.String() {
		if !IsCommandAvailable(wgWinBinary) {
			return nil, fmt.Errorf("wireguard.exe command not found, is wireguard installed?")
		}
	} else {
		if !IsCommandAvailable(wgBinary) {
			return nil, fmt.Errorf("wg command not found, is wireguard installed?")
		}
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	return ax, nil
}

func (ax *Apex) Run() {
	ctx := context.Background()
	var err error

	if err := ax.handleKeys(); err != nil {
		log.Fatalf("handleKeys: %+v", err)
	}

	if ax.withToken == "" {
		ax.auth, err = NewDeviceFlowAuthenticator(ctx, ax.controllerURL)
		if err != nil {
			log.Fatalf("authentication error: %+v", err)
		}
	} else {
		ax.auth = &TokenAuthenticator{accessToken: ax.withToken}
	}

	token, err := ax.auth.Token()
	if err != nil {
		log.Fatalf("can't get auth token: %s", err)
	}
	var deviceID string
	if deviceID, err = RegisterDevice(ax.controllerURL, ax.wireguardPubKey, token); err != nil {
		log.Fatalf("device register error: %+v", err)
	}
	log.Infof("Device Registered with UUID: %s", deviceID)

	if ax.zone, err = GetZone(ax.controllerURL, token); err != nil {
		log.Fatalf("get zone error: %+v", err)
	}
	log.Infof("Device belongs in zone: %s", ax.zone)

	var localEndpointIP string
	// User requested ip --request-ip takes precedent
	if ax.userProvidedEndpointIP != "" {
		localEndpointIP = ax.userProvidedEndpointIP
	}
	if ax.stun && localEndpointIP == "" {
		localEndpointIP, err = GetPubIP()
		if err != nil {
			log.Warn("Unable to determine the public facing address, falling back to the local address")
		}
	}
	if localEndpointIP == "" {
		localEndpointIP, err = ax.findLocalEndpointIp()
		if err != nil {
			log.Fatalf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %v", err)
		}
	}
	ax.localEndpointIP = localEndpointIP
	log.Infof("This node's endpoint address for building tunnels is [ %s ]", ax.localEndpointIP)

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, WgListenPort)

	registerRequest := Peer{
		DeviceID:    deviceID,
		EndpointIP:  endpointSocket,
		NodeAddress: ax.requestedIP,
		ChildPrefix: ax.childPrefix,
		HubRouter:   ax.hubRouter,
		HubZone:     false,
		ZonePrefix:  "",
	}

	data, err := json.Marshal(registerRequest)
	if err != nil {
		log.Fatalf("marshalling error: %+v", err)
	}

	dest, err := url.JoinPath(ax.controllerURL.String(), fmt.Sprintf(REGISTER_URL, ax.zone))
	if err != nil {
		log.Fatalf("unable to create dest url: %s", err)
	}
	req, err := http.NewRequest("POST", dest, bytes.NewReader(data))
	if err != nil {
		log.Fatalf("cannot create register request error: %+v", err)
	}
	req.Header.Set("authorization", fmt.Sprintf("bearer %s", token))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("cannot send register request error: %+v", err)
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != http.StatusCreated {
		log.Fatalf("unsuccessful register request: %d %s", res.StatusCode, resBody)
	}

	log.Info("Sucessfully registered with Apex Controller")

	// a hub router requires ip forwarding and iptables rules, OS type has already been checked
	if ax.hubRouter {
		enableForwardingIPv4()
		hubRouterIpTables()
	}

	if err := ax.Reconcile(); err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(pollInterval)
	for range ticker.C {
		if err := ax.Reconcile(); err != nil {
			// TODO: Add smarter reconciliation logic
			// to handle disconnects and/or timeouts etc...
			log.Fatal(err)
		}
	}
}

func (ax *Apex) Reconcile() error {
	dest, err := url.JoinPath(ax.controllerURL.String(), fmt.Sprintf(REGISTER_URL, ax.zone))
	if err != nil {
		log.Fatalf("unable to create dest url: %s", err)
	}
	token, err := ax.auth.Token()
	if err != nil {
		log.Fatalf("unable to get auth token: %s", err)
	}
	req, err := http.NewRequest("GET", dest, nil)
	if err != nil {
		return fmt.Errorf("cannot create peer request error: %+v", err)
	}
	req.Header.Set("authorization", fmt.Sprintf("bearer %s", token))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot create peer request error: %+v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("cannot read body: %+v", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("http error: %s", string(body))
	}

	var peerListing []Peer
	if err := json.Unmarshal(body, &peerListing); err != nil {
		return fmt.Errorf("cannot unmarshal body: %+v", err)
	}

	log.Debugf("Received message: %+v\n", peerListing)

	changed := false
	for _, p := range peerListing {
		existing, ok := ax.peerCache[p.ID]
		if !ok {
			changed = true
			ax.peerCache[p.ID] = p
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			ax.peerCache[p.ID] = p
		}
	}

	if changed {
		log.Debugf("Peer listing has changed, recalculating configuration")
		ax.ParseWireguardConfig(ax.listenPort)
		ax.DeployWireguardConfig()
	}
	return nil
}

func (ax *Apex) Shutdown(ctx context.Context) error {
	return nil
}

// Check OS and report error if the OS is not supported.
func checkOS() error {
	nodeOS := GetOS()
	switch nodeOS {
	case Darwin.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the osx wireguard directory exists
		if err := CreateDirectory(WgDarwinConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgDarwinConfPath, err)
		}
		if ifaceExists(darwinIface) {
			deleteDarwinIface()
		}
	case Windows.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the windows wireguard directory exists
		if err := CreateDirectory(WgWindowsConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgWindowsConfPath, err)
		}
	case Linux.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the linux wireguard directory exists
		if err := CreateDirectory(WgLinuxConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgLinuxConfPath, err)
		}
	default:
		return fmt.Errorf("OS [%s] is not supported\n", nodeOS)
	}
	return nil
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (ax *Apex) checkUnsupportedConfigs() error {
	if ax.hubRouter && ax.os == Darwin.String() {
		log.Fatalf("OSX nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.hubRouter && ax.os == Windows.String() {
		log.Fatalf("Windows nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.userProvidedEndpointIP != "" {
		if err := ValidateIp(ax.userProvidedEndpointIP); err != nil {
			log.Fatalf("the IP address passed in --local-endpoint-ip %s was not valid: %v", ax.userProvidedEndpointIP, err)
		}
	}
	if ax.requestedIP != "" {
		if err := ValidateIp(ax.requestedIP); err != nil {
			log.Fatalf("the IP address passed in --request-ip %s was not valid: %v", ax.requestedIP, err)
		}
	}
	if ax.childPrefix != "" {
		if err := ValidateCIDR(ax.childPrefix); err != nil {
			log.Fatalf("the CIDR prefix passed in --child-prefix %s was not valid: %v", ax.childPrefix, err)
		}
	}
	// replace the interface with the newly assigned interface
	if ax.os == Linux.String() && linkExists(wgIface) {
		if err := delLink(wgIface); err != nil {
			// not a fatal error since if this is on startup it could be absent
			log.Debugf("failed to delete netlink interface %s: %v", wgIface, err)
		}
	}
	if ax.os == Darwin.String() {
		if ifaceExists(darwinIface) {
			deleteDarwinIface()
		}
	}
	return nil
}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	// Darwin network discovery
	if !ax.stun && ax.os == Darwin.String() {
		controllerHost, controllerPort, err := net.SplitHostPort(ax.controllerURL.Host)
		if err != nil {
			log.Errorf("failed to split host:port endpoint pair: %v", err)
		}
		localEndpointIP, err = discoverGenericIPv4(controllerHost, controllerPort)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Windows network discovery
	if !ax.stun && ax.os == Windows.String() {
		controllerHost, controllerPort, err := net.SplitHostPort(ax.controllerURL.Host)
		if err != nil {
			log.Errorf("failed to split host:port endpoint pair: %v", err)
		}
		localEndpointIP, err = discoverGenericIPv4(controllerHost, controllerPort)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Linux network discovery
	if !ax.stun && ax.os == Linux.String() {
		linuxIP, err := discoverLinuxAddress(4)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
		localEndpointIP = linuxIP.String()
	}
	return localEndpointIP, nil
}
