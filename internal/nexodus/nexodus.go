package nexodus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/stun"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.org/x/term"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

const (
	pollInterval       = 5 * time.Second
	wgGoBinary         = "wireguard-go"
	nexdWgGoBinary     = "nexd-wireguard-go"
	wgWinBinary        = "wireguard.exe"
	WgLinuxConfPath    = "/etc/wireguard/"
	WgDarwinConfPath   = "/usr/local/etc/wireguard/"
	darwinIface        = "utun8"
	WgDefaultPort      = 51820
	wgIface            = "wg0"
	WgWindowsConfPath  = "C:/nexd/"
	wgOrgIPv6PrefixLen = "64"
	apiToken           = "apitoken.json"
)

const (
	// when nexd is first starting up
	NexdStatusStarting = iota
	// when nexd is waiting for auth and the user must complete the OTP auth flow
	NexdStatusAuth
	// nexd is up and running normally
	NexdStatusRunning
)

const (
	// retry interval for api server retries
	retryInterval = 15 * time.Second
	// max retries for api server retries
	maxRetries = 3
)

var (
	invalidTokenGrant = errors.New("invalid_grant")
	invalidToken      = errors.New("invalid_token")
)

// embedded in Nexodus struct
type userspaceWG struct {
	userspaceMode bool
	userspaceTun  tun.Device
	userspaceNet  *netstack.Net
	userspaceDev  *device.Device
	// the last address configured on the userspace wireguard interface
	userspaceLastAddress string
	proxyLock            sync.RWMutex
	proxies              map[ProxyKey]*UsProxy
}

type deviceCacheEntry struct {
	device public.ModelsDevice
	// the last time this device was updated
	lastUpdated time.Time
}

type Nexodus struct {
	wireguardPubKey          string
	wireguardPvtKey          string
	wireguardPubKeyInConfig  bool
	tunnelIface              string
	controllerIP             string
	listenPort               int
	orgId                    string
	org                      *public.ModelsOrganization
	requestedIP              string
	userProvidedLocalIP      string
	TunnelIP                 string
	TunnelIpV6               string
	childPrefix              []string
	stun                     bool
	relay                    bool
	relayWgIP                string
	wgConfig                 wgConfig
	client                   *client.APIClient
	controllerURL            *url.URL
	deviceCache              map[string]deviceCacheEntry
	endpointLocalAddress     string
	nodeReflexiveAddressIPv4 netip.AddrPort
	hostname                 string
	securityGroup            *public.ModelsSecurityGroup
	symmetricNat             bool
	ipv6Supported            bool
	os                       string
	logger                   *zap.SugaredLogger
	logLevel                 *zap.AtomicLevel
	// See the NexdStatus* constants
	status        int
	statusMsg     string
	version       string
	username      string
	password      string
	skipTlsVerify bool
	stateDir      string
	userspaceWG
	informer     *public.ApiListDevicesInOrganizationInformer
	informerStop context.CancelFunc
	nexCtx       context.Context
	nexWg        *sync.WaitGroup
}

type wgConfig struct {
	Interface wgLocalConfig
	Peers     map[string]wgPeerConfig
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
}

type wgLocalConfig struct {
	PrivateKey string
	ListenPort int
}

func NewNexodus(
	logger *zap.SugaredLogger,
	logLevel *zap.AtomicLevel,
	controller string,
	username string,
	password string,
	wgListenPort int,
	wireguardPubKey string,
	wireguardPvtKey string,
	requestedIP string,
	userProvidedLocalIP string,
	childPrefix []string,
	stun bool,
	relay bool,
	relayOnly bool,
	insecureSkipTlsVerify bool,
	version string,
	userspaceMode bool,
	stateDir string,
	ctx context.Context,
	orgId string,
) (*Nexodus, error) {

	if err := binaryChecks(); err != nil {
		return nil, err
	}

	controllerURL, err := url.Parse(controller)
	if err != nil {
		return nil, fmt.Errorf("invalid controller-url provided: %s error: %w, please use the following format https://<controller-url>", controller, err)
	}

	if !strings.HasPrefix(controller, "https://") {
		return nil, fmt.Errorf("invalid controller-url provided: %s, please use the following format https://<controller-url>", controller)
	}

	// Force controller URL be api.${DOMAIN}
	controllerURL.Host = "api." + controllerURL.Host
	controllerURL.Path = ""

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	if wgListenPort == 0 {
		wgListenPort, err = getWgListenPort()
		if err != nil {
			return nil, err
		}
	}

	ax := &Nexodus{
		wireguardPubKey:     wireguardPubKey,
		wireguardPvtKey:     wireguardPvtKey,
		controllerIP:        controller,
		listenPort:          wgListenPort,
		requestedIP:         requestedIP,
		userProvidedLocalIP: userProvidedLocalIP,
		childPrefix:         childPrefix,
		stun:                stun,
		relay:               relay,
		deviceCache:         make(map[string]deviceCacheEntry),
		controllerURL:       controllerURL,
		hostname:            hostname,
		symmetricNat:        relayOnly,
		logger:              logger,
		logLevel:            logLevel,
		status:              NexdStatusStarting,
		version:             version,
		username:            username,
		password:            password,
		skipTlsVerify:       insecureSkipTlsVerify,
		stateDir:            stateDir,
		orgId:               orgId,
		userspaceWG: userspaceWG{
			proxies: map[ProxyKey]*UsProxy{},
		},
	}
	ax.userspaceMode = userspaceMode
	ax.tunnelIface = ax.defaultTunnelDev()

	if ax.relay {
		ax.listenPort = WgDefaultPort
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	if err := prepOS(logger); err != nil {
		return nil, err
	}

	// remove orphaned wg interfaces from previous node joins
	ax.removeExistingInterface()

	if err := ax.symmetricNatDisco(ctx); err != nil {
		ax.logger.Warn(err)
	}

	return ax, nil
}

func (ax *Nexodus) SetStatus(status int, msg string) {
	ax.statusMsg = msg
	ax.status = status
}

func (ax *Nexodus) Start(ctx context.Context, wg *sync.WaitGroup) error {
	ax.nexCtx = ctx
	ax.nexWg = wg

	// Block additional proxy configuration coming in via the ctl server until after
	// initial startup is complete.
	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	var err error
	if err := ax.CtlServerStart(ctx, wg); err != nil {
		return fmt.Errorf("CtlServerStart(): %w", err)
	}

	if runtime.GOOS != Linux.String() {
		ax.logger.Info("Security Groups are currently only supported on Linux")
	} else if ax.userspaceMode {
		ax.logger.Info("Security Groups are not supported in userspace proxy mode")
	}

	var options []client.Option
	if ax.stateDir != "" {
		options = append(options, client.WithTokenFile(filepath.Join(ax.stateDir, apiToken)))
	}
	if ax.username == "" {
		options = append(options, client.WithDeviceFlow())
	} else if ax.username != "" && ax.password == "" {
		fmt.Print("Enter nexodus account password: ")
		passwdInput, err := term.ReadPassword(int(syscall.Stdin))
		println()
		if err != nil {
			return fmt.Errorf("login aborted: %w", err)
		}
		ax.password = string(passwdInput)
		options = append(options, client.WithPasswordGrant(ax.username, ax.password))
	} else {
		options = append(options, client.WithPasswordGrant(ax.username, ax.password))
	}
	if ax.skipTlsVerify { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}

	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		ax.client, err = client.NewAPIClient(ctx, ax.controllerURL.String(), func(msg string) {
			ax.SetStatus(NexdStatusAuth, msg)
		}, options...)
		if err != nil {
			ax.logger.Warnf("client api error - retrying: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("client api error: %w", err)
	}

	ax.SetStatus(NexdStatusRunning, "")

	if err := ax.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	var user *public.ModelsUser
	var resp *http.Response
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		user, resp, err = ax.client.UsersApi.GetUser(ctx, "me").Execute()
		if err != nil {
			if resp != nil {
				ax.logger.Warnf("get user error - retrying error: %v header: %+v", err, resp.Header)
				return err
			} else if strings.Contains(err.Error(), invalidTokenGrant.Error()) || strings.Contains(err.Error(), invalidToken.Error()) {
				ax.logger.Errorf("The nexodus token stored in %s/%s is not valid for the api-server, you can remove the file and try again: %v", ax.stateDir, apiToken, err)
				return err
			} else {
				ax.logger.Warnf("get user error - retrying error: %v", err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("get user error: %w", err)
	}

	var organizations []public.ModelsOrganization
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		organizations, resp, err = ax.client.OrganizationsApi.ListOrganizations(ctx).Execute()
		if err != nil {
			if resp != nil {
				ax.logger.Warnf("get organizations error - retrying error: %v header: %+v", err, resp.Header)
				return err
			}
			if err != nil {
				ax.logger.Warnf("get organizations error - retrying error: %v", err)
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("get organizations error: %w", err)
	}

	ax.org, err = ax.chooseOrganization(organizations, *user)
	if err != nil {
		return fmt.Errorf("failed to choose an organization: %w", err)
	}

	informerCtx, informerCancel := context.WithCancel(ctx)
	ax.informerStop = informerCancel
	ax.informer = ax.client.DevicesApi.ListDevicesInOrganization(informerCtx, ax.org.Id).Informer()

	var localIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if ax.userProvidedLocalIP != "" {
		localIP = ax.userProvidedLocalIP
		localEndpointPort = ax.listenPort
	}

	if ax.relay {
		peerMap, _, err := ax.informer.Execute()
		if err != nil {
			return err
		}

		existingRelay, err := ax.orgRelayCheck(peerMap)
		if err != nil {
			return err
		}
		if existingRelay != "" {
			return fmt.Errorf("the organization already contains a relay node, device %s needs to be deleted before adding a new relay", existingRelay)
		}
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	stunServer1 := stun.NextServer()
	if !ax.symmetricNat && ax.stun && localIP == "" {
		ipPort, err := stun.Request(ax.logger, stunServer1, ax.listenPort)
		if err != nil {
			ax.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.Addr().String()
			localEndpointPort = int(ipPort.Port())
		}
	}
	if localIP == "" {
		ip, err := ax.findLocalIP()
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localIP = ip
		localEndpointPort = ax.listenPort
	}

	ax.os = runtime.GOOS

	ax.endpointLocalAddress = localIP
	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	endpoints := []public.ModelsEndpoint{
		{
			Source:   "local",
			Address:  endpointSocket,
			Distance: 0,
		},
		{
			Source:   "stun:" + stunServer1,
			Address:  ax.nodeReflexiveAddressIPv4.String(),
			Distance: 0,
		},
	}

	var modelsDevice public.ModelsDevice
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		modelsDevice, err = ax.createOrUpdateDeviceOperation(user.Id, endpoints)
		if err != nil {
			ax.logger.Warnf("device join error - retrying: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("join error %w", err)
	}

	ax.logger.Debug(fmt.Sprintf("Device: %+v", modelsDevice))
	ax.logger.Infof("Successfully registered device with UUID: [ %+v ] into organization: [ %s (%s) ]",
		modelsDevice.Id, ax.org.Name, ax.org.Id)

	// a relay node requires ip forwarding and nftable rules, OS type has already been checked
	if ax.relay {
		if err := ax.relayPrep(); err != nil {
			return err
		}
	}

	util.GoWithWaitGroup(wg, func() {
		// kick it off with an immediate reconcile
		ax.reconcileDevices(ctx, options)
		ax.reconcileSecurityGroups(ctx)
		for _, proxy := range ax.proxies {
			proxy.Start(ctx, wg, ax.userspaceNet)
		}
		stunTicker := time.NewTicker(time.Second * 20)
		secGroupTicker := time.NewTicker(time.Second * 20)
		defer stunTicker.Stop()
		pollTicker := time.NewTicker(pollInterval)
		defer pollTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stunTicker.C:
				if err := ax.reconcileStun(modelsDevice.Id); err != nil {
					ax.logger.Debug(err)
				}
			case <-ax.informer.Changed():
				ax.reconcileDevices(ctx, options)
			case <-pollTicker.C:
				// This does not actually poll the API for changes. Peer configuration changes will only
				// be processed when they come in on the informer. This periodic check is needed to
				// re-establish our connection to the API if it is lost.
				ax.reconcileDevices(ctx, options)
			case <-secGroupTicker.C:
				ax.reconcileSecurityGroups(ctx)
			}
		}
	})

	return nil
}

func (ax *Nexodus) Stop() {
	ax.logger.Info("Stopping nexd")
	for _, proxy := range ax.proxies {
		proxy.Stop()
	}
}

// reconcileSecurityGroups will check the security group and update it if necessary.
func (ax *Nexodus) reconcileSecurityGroups(ctx context.Context) {
	if runtime.GOOS != Linux.String() || ax.userspaceMode {
		return
	}

	existing, ok := ax.deviceCache[ax.wireguardPubKey]
	if !ok {
		// local device not in the cache, so we don't have our config yet.
		return
	}

	if existing.device.SecurityGroupId == uuid.Nil.String() {
		// local device has no security group
		if ax.securityGroup == nil {
			// already set up that way, nothing to do
			return
		}
		// drop local security group configuration
		ax.securityGroup = nil
		if err := ax.processSecurityGroupRules(); err != nil {
			ax.logger.Error(err)
		}
		return
	}

	// if the security group ID is not nil, lookup the ID and check for any changes
	responseSecGroup, httpResp, err := ax.client.SecurityGroupApi.GetSecurityGroup(ctx, ax.org.Id, existing.device.SecurityGroupId).Execute()
	if err != nil {
		// if the group ID returns a 404, clear the current rules
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			ax.securityGroup = nil
			if err := ax.processSecurityGroupRules(); err != nil {
				ax.logger.Error(err)
			}
			return
		}
		ax.logger.Errorf("Error retrieving the security group: %v", err)
		return
	}

	if ax.securityGroup != nil && reflect.DeepEqual(responseSecGroup, ax.securityGroup) {
		// no changes to previously applied security group
		return
	}

	ax.logger.Debugf("Security Group change detected: %+v", responseSecGroup)
	oldSecGroup := ax.securityGroup
	ax.securityGroup = responseSecGroup

	if oldSecGroup != nil && responseSecGroup.Id == oldSecGroup.Id &&
		reflect.DeepEqual(responseSecGroup.InboundRules, oldSecGroup.InboundRules) &&
		reflect.DeepEqual(responseSecGroup.OutboundRules, oldSecGroup.OutboundRules) {
		// the group changed, but not in a way that matters for applying the rules locally
		return
	}

	// apply the new security group rules
	if err := ax.processSecurityGroupRules(); err != nil {
		ax.logger.Error(err)
	}
}

func (ax *Nexodus) reconcileDevices(ctx context.Context, options []client.Option) {
	var err error
	if err = ax.Reconcile(); err == nil {
		return
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.Temporary() {
		// Temporary dns resolution failure is normal, just debug log it
		ax.logger.Debugf("%v", err)
		return
	}

	ax.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)

	// if the token grant becomes invalid expires refresh or exit depending on the onboard method
	if !strings.Contains(err.Error(), invalidTokenGrant.Error()) {
		return
	}

	// token grant has become invalid, if we are using a one-time auth token, exit
	if ax.username == "" {
		ax.logger.Fatalf("The token grant has expired due to an extended period offline, please " +
			"restart the agent for a one-time auth or login with --username --password to automatically reconnect")
		return
	}

	// do we need to stop the informer?
	if ax.informerStop != nil {
		ax.informerStop()
		ax.informerStop = nil
	}

	// refresh the token grant by reconnecting to the API server
	c, err := client.NewAPIClient(ctx, ax.controllerURL.String(), func(msg string) {
		ax.SetStatus(NexdStatusAuth, msg)
	}, options...)
	if err != nil {
		ax.logger.Errorf("Failed to reconnect to the api-server, retrying in %v seconds: %v", pollInterval, err)
		return
	}

	ax.client = c
	informerCtx, informerCancel := context.WithCancel(ctx)
	ax.informerStop = informerCancel
	ax.informer = ax.client.DevicesApi.ListDevicesInOrganization(informerCtx, ax.org.Id).Informer()

	ax.SetStatus(NexdStatusRunning, "")
	ax.logger.Infoln("Nexodus agent has re-established a connection to the api-server")
}

func (ax *Nexodus) reconcileStun(deviceID string) error {
	if ax.symmetricNat {
		return nil
	}

	ax.logger.Debug("sending stun request")
	stunServer1 := stun.NextServer()
	reflexiveIP, err := stun.Request(ax.logger, stunServer1, ax.listenPort)
	if err != nil {
		return fmt.Errorf("stun request error: %w", err)
	}

	if ax.nodeReflexiveAddressIPv4 != reflexiveIP {
		ax.logger.Infof("detected a NAT binding changed for this device %s from %s to %s, updating peers", deviceID, ax.nodeReflexiveAddressIPv4, reflexiveIP)

		res, _, err := ax.client.DevicesApi.UpdateDevice(context.Background(), deviceID).Update(public.ModelsUpdateDevice{
			Endpoints: []public.ModelsEndpoint{
				{
					Source:   "local",
					Address:  net.JoinHostPort(ax.endpointLocalAddress, fmt.Sprintf("%d", ax.listenPort)),
					Distance: 0,
				},
				{
					Source:   "stun:" + stunServer1,
					Address:  reflexiveIP.String(),
					Distance: 0,
				},
			},
		}).Execute()
		if err != nil {
			return fmt.Errorf("failed to update this device's new NAT binding, likely still reconnecting to the api-server, retrying in 20s: %w", err)
		} else {
			ax.logger.Debugf("update device response %+v", res)
			ax.nodeReflexiveAddressIPv4 = reflexiveIP
			// reinitialize peers if the NAT binding has changed for the node
			if err = ax.Reconcile(); err != nil {
				ax.logger.Debugf("reconcile failed %v", res)
			}
		}
	}

	return nil
}

func (nx *Nexodus) chooseOrganization(organizations []public.ModelsOrganization, user public.ModelsUser) (*public.ModelsOrganization, error) {
	if len(organizations) == 0 {
		return nil, fmt.Errorf("user does not belong to any organizations")
	}
	if nx.orgId == "" {
		if len(organizations) > 1 {
			// default to the org that matches the user name, the one created for a new user by default
			for i, org := range organizations {
				if org.Name == user.UserName {
					return &organizations[i], nil
				}
			}
			// Log all org names + Ids for convenience before returning the error
			for _, org := range organizations {
				nx.logger.Infof("organization name: '%s'  Id: %s", org.Name, org.Id)
			}
			return nil, fmt.Errorf("user belongs to multiple organizations, please specify one with --org-id")
		}
		return &organizations[0], nil
	}
	for i, org := range organizations {
		if org.Id == nx.orgId {
			return &organizations[i], nil
		}
	}
	return nil, fmt.Errorf("user does not belong to organization %s", nx.orgId)
}

func (nx *Nexodus) addToDeviceCache(p public.ModelsDevice) {
	nx.deviceCache[p.PublicKey] = deviceCacheEntry{
		device:      p,
		lastUpdated: time.Now(),
	}
}

func (ax *Nexodus) Reconcile() error {
	peerMap, resp, err := ax.informer.Execute()
	if err != nil {
		if resp != nil {
			return fmt.Errorf("error: %w header: %v", err, resp.Header)
		}
		return fmt.Errorf("error: %w", err)
	}

	newLocalConfig := false
	for _, p := range peerMap {
		existing, ok := ax.deviceCache[p.PublicKey]
		if !ok || !ax.isEqualIgnoreSecurityGroup(existing.device, p) {
			if p.PublicKey == ax.wireguardPubKey {
				newLocalConfig = true
			}
			ax.addToDeviceCache(p)
		}
		if p.Relay {
			ax.relayWgIP = p.AllowedIps[0]
		}
	}

	updatePeers := ax.buildPeersConfig()
	if newLocalConfig || len(updatePeers) > 0 {
		if err := ax.DeployWireguardConfig(updatePeers); err != nil {
			if strings.Contains(err.Error(), securityGroupErr.Error()) {
				return err
			}
			// If the wireguard configuration fails, we should wipe out our peer list
			// so it is rebuilt and reconfigured from scratch the next time around.
			ax.wgConfig.Peers = nil
			return err
		}
	}

	// check for any peer deletions
	if err := ax.handlePeerDelete(peerMap); err != nil {
		ax.logger.Error(err)
	}

	return nil
}

func (ax *Nexodus) isEqualIgnoreSecurityGroup(p1, p2 public.ModelsDevice) bool {
	// create temporary copies of the instances
	tmpDev1 := p1
	tmpDev2 := p2
	// set the SecurityGroupId to an empty value, so it will not affect the comparison
	tmpDev1.SecurityGroupId = ""
	tmpDev2.SecurityGroupId = ""
	// set the Revision to 0, so it will not affect the comparison
	tmpDev1.Revision = 0
	tmpDev2.Revision = 0

	return reflect.DeepEqual(tmpDev1, tmpDev2)
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (ax *Nexodus) checkUnsupportedConfigs() error {

	if ax.ipv6Supported = isIPv6Supported(); !ax.ipv6Supported {
		ax.logger.Warn("IPv6 does not appear to be enabled on this host, only IPv4 will be provisioned or restart nexd with IPv6 enabled on this host")
	}

	return nil
}

// symmetricNatDisco determine if the joining node is within a symmetric NAT cone
func (ax *Nexodus) symmetricNatDisco(ctx context.Context) error {

	stunRetryTimer := time.Second * 1
	err := util.RetryOperation(ctx, stunRetryTimer, maxRetries, func() error {
		stunServer1 := stun.NextServer()
		stunServer2 := stun.NextServer()
		stunAddr1, err := stun.Request(ax.logger, stunServer1, ax.listenPort)
		if err != nil {
			return err
		} else {
			ax.nodeReflexiveAddressIPv4 = stunAddr1
		}

		isSymmetric := false
		stunAddr2, err := stun.Request(ax.logger, stunServer2, ax.listenPort)
		if err != nil {
			return err
		} else {
			isSymmetric = stunAddr1.String() != stunAddr2.String()
		}

		if stunAddr1.Addr().String() != "" {
			ax.logger.Debugf("first NAT discovery STUN request returned: %s", stunAddr1.String())
		} else {
			ax.logger.Debugf("first NAT discovery STUN request returned an empty value")
		}

		if stunAddr2.Addr().String() != "" {
			ax.logger.Debugf("second NAT discovery STUN request returned: %s", stunAddr2.String())
		} else {
			ax.logger.Debugf("second NAT discovery STUN request returned an empty value")
		}

		if isSymmetric {
			ax.symmetricNat = true
			ax.logger.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("STUN discovery error: %w", err)
	}

	return nil
}

// orgRelayCheck checks if there is an existing Relay node in the organization that does not match this device's pub key
func (ax *Nexodus) orgRelayCheck(peerMap map[string]public.ModelsDevice) (string, error) {
	for _, p := range peerMap {
		if p.Relay && ax.wireguardPubKey != p.PublicKey {
			return p.Id, nil
		}
	}
	return "", nil
}

func (ax *Nexodus) setupInterface() error {
	if ax.userspaceMode {
		return ax.setupInterfaceUS()
	}
	return ax.setupInterfaceOS()
}

func (ax *Nexodus) defaultTunnelDev() string {
	if ax.userspaceMode {
		return ax.defaultTunnelDevUS()
	}
	return defaultTunnelDevOS()
}
