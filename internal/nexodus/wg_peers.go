package nexodus

import (
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	persistentKeepalive = "20"             // seconds
	localPeeringTimeout = 30 * time.Second // longer than persistentKeepalive
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Nexodus) buildPeersConfig(updatedDevices map[string]public.ModelsDevice) []string {
	peers, changedPeers := ax.buildPeersAndRelay(updatedDevices)
	ax.wgConfig.Peers = peers
	return changedPeers
}

func isWorking(peer WgSessions) bool {
	return peer.LastHandshakeTime.After(time.Now().Add(-localPeeringTimeout))
}

// localPeerCandidate determines whether this peer is a candidate for peering via its local address.
func (nx *Nexodus) localPeerCandidate(d deviceCacheEntry, peerStats map[string]WgSessions, reflexiveIP4 string) bool {
	// We must be behind the same reflexive address as the peer
	if nx.nodeReflexiveAddressIPv4.Addr().String() != parseIPfromAddrPort(reflexiveIP4) {
		nx.logger.Infof("Peer %s is not behind the same reflexive address as us", d.device.PublicKey)
		return false
	}

	// We've already fallen back to reflexive IP peering
	if d.reflexivePeeringFallback {
		nx.logger.Infof("Peer %s has already fallen back to reflexive IP peering", d.device.PublicKey)
		return false
	}

	// Determine whether we tried, it hasn't worked, and we should fall back to our next best option
	window := time.Now().Add(-localPeeringTimeout)

	// cache entry is old enough (we have given it enough time to try and connect)
	// and the last handshake time is too long ago
	stats, ok := peerStats[d.device.PublicKey]
	if !ok {
		nx.logger.Debugf("Peer %s not in peerStats: %s", d.device.PublicKey, peerStats)
	}

	if ok && window.After(d.lastUpdated) && window.After(stats.LastHandshakeTime) {
		nx.logger.Debugf("Peer %s has not connected via local IP, falling back to reflexive IP", d.device.PublicKey)
		nx.deviceCache[d.device.PublicKey] = deviceCacheEntry{
			device:                   d.device,
			reflexivePeeringFallback: true,
			lastUpdated:              time.Now(),
		}
		return false
	}

	return true
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, aVal := range a {
		found := false
		for _, bVal := range b {
			if aVal == bVal {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// buildPeersAndRelay builds the peer configuration based off peer cache and local state.
// This also calls the method for building the local interface configuration wgLocalConfig.
// If changed is true, this was called as a result of a change to the device cache.
func (ax *Nexodus) buildPeersAndRelay(updatedDevices map[string]public.ModelsDevice) (map[string]wgPeerConfig, []string) {
	// full config map
	peers := map[string]wgPeerConfig{}
	// whether any devices have changed
	changed := len(updatedDevices) > 0
	// track which peers we changed configuration for
	changedPeers := []string{}

	_, ax.wireguardPubKeyInConfig = ax.deviceCache[ax.wireguardPubKey]

	relayAllowedIP := []string{
		ax.org.Cidr,
		ax.org.CidrV6,
	}

	ax.buildLocalConfig()
	peerStats, err := ax.DumpPeers(ax.tunnelIface)
	if err != nil {
		ax.logger.Errorf("Failed to get current peer details from wg: %w", err)
		peerStats = make(map[string]WgSessions)
	}
	for _, d := range ax.deviceCache {
		// skip ourselves
		if d.device.PublicKey == ax.wireguardPubKey {
			continue
		}

		localIP, reflexiveIP4 := ax.extractLocalAndReflexiveIP(d.device)
		peerPort := ax.extractPeerPort(localIP)

		// We are a relay node. This block will get hit for every peer.
		if ax.relay {
			peer := ax.buildPeerForRelayNode(d.device, localIP, reflexiveIP4)
			peers[d.device.PublicKey] = peer
			ax.logPeerInfo(d.device, reflexiveIP4, changed)
			continue
		}

		// The peer is a relay node
		if d.device.Relay {
			peerRelay := ax.buildRelayPeer(d.device, relayAllowedIP, localIP, reflexiveIP4)
			peers[d.device.PublicKey] = peerRelay
			ax.logPeerInfo(d.device, peerRelay.Endpoint, changed)
			continue
		}

		// If it's working and the config has not changed, leave it alone.
		curStats, ok := peerStats[d.device.PublicKey]
		if ok && isWorking(curStats) {
			_, deviceChanged := updatedDevices[d.device.PublicKey]
			if !deviceChanged {
				// it's working and nothing changed in the device config
				continue
			}
			// did something change that matters?
			curConfig, ok := ax.wgConfig.Peers[d.device.PublicKey]
			if ok && equalSets(curConfig.AllowedIPs, d.device.AllowedIps) &&
				(curConfig.Endpoint == reflexiveIP4 || curConfig.Endpoint == localIP) {
				// AllowedIPs still the same, and the endpoint is still valid
				continue
			}
			// It's working, but something changed. We need to reconfigure it.
		}

		// Determine if this is a candidate for attempting to peer via its local address
		if ax.localPeerCandidate(d, peerStats, reflexiveIP4) {
			peer := ax.buildDirectLocalPeer(d.device, localIP, peerPort)
			peers[d.device.PublicKey] = peer
			ax.logPeerInfo(d.device, localIP, changed)
			continue
		}

		// If we are behind symmetric NAT, we have no further options
		if ax.symmetricNat {
			continue
		}

		// If the peer is not behind symmetric NAT, we can try peering with its reflexive address
		if !d.device.SymmetricNat {
			peer := ax.buildDefaultPeer(d.device, reflexiveIP4)
			peers[d.device.PublicKey] = peer
			ax.logPeerInfo(d.device, reflexiveIP4, changed)
		}
	}

	return peers, changedPeers
}

// extractLocalAndReflexiveIP retrieve the local and reflexive endpoint addresses
func (ax *Nexodus) extractLocalAndReflexiveIP(device public.ModelsDevice) (string, string) {
	localIP := ""
	reflexiveIP4 := ""
	for _, endpoint := range device.Endpoints {
		if endpoint.Source == "local" {
			localIP = endpoint.Address
		} else {
			reflexiveIP4 = endpoint.Address
		}
	}
	return localIP, reflexiveIP4
}

func (ax *Nexodus) extractPeerPort(localIP string) string {
	_, port, err := net.SplitHostPort(localIP)
	if err != nil {
		ax.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v", err)
		return ""
	}
	return port
}

// buildRelayPeer Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
// This is the only peer a symmetric NAT node will get unless it also has a direct peering
func (ax *Nexodus) buildRelayPeer(device public.ModelsDevice, relayAllowedIP []string, localIP, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	config := wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
	if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) {
		config.Endpoint = localIP
	}
	return config
}

// buildPeerForRelayNode build a config for all peers if this node is the organization's relay node. Also check for direct peering.
// The peer for a relay node is currently left blank and assumed to be exposed to all peers, we still build its peer config for flexibility.
func (ax *Nexodus) buildPeerForRelayNode(device public.ModelsDevice, localIP, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	config := wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
	if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) {
		config.Endpoint = localIP
	}
	return config
}

// buildDirectLocalPeer If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
func (ax *Nexodus) buildDirectLocalPeer(device public.ModelsDevice, localIP, peerPort string) wgPeerConfig {
	directLocalPeerEndpointSocket := net.JoinHostPort(device.EndpointLocalAddressIp4, peerPort)
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            directLocalPeerEndpointSocket,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDefaultPeer the bulk of the peers will be added here except for local address peers or
// symmetric NAT peers or if this device is itself a symmetric nat node, that require relaying.
func (ax *Nexodus) buildDefaultPeer(device public.ModelsDevice, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func (ax *Nexodus) logPeerInfo(device public.ModelsDevice, endpointIP string, changed bool) {
	if !changed {
		return
	}
	ax.logger.Debugf("Peer Configuration - Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]",
		strings.Join(device.AllowedIps, ", "),
		endpointIP,
		device.PublicKey)
}

// buildLocalConfig builds the configuration for the local interface
func (ax *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig
	var d deviceCacheEntry
	var ok bool

	if d, ok = ax.deviceCache[ax.wireguardPubKey]; !ok {
		return
	}

	// if the local node address changed replace it on wg0
	if ax.TunnelIP != d.device.TunnelIp {
		ax.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", d.device.TunnelIp, d.device.TunnelIpV6)
		if runtime.GOOS == Linux.String() && linkExists(ax.tunnelIface) {
			if err := delLink(ax.tunnelIface); err != nil {
				ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
			}
		}
	}
	ax.TunnelIP = d.device.TunnelIp
	ax.TunnelIpV6 = d.device.TunnelIpV6
	localInterface = wgLocalConfig{
		ax.wireguardPvtKey,
		ax.listenPort,
	}
	ax.logger.Debugf("Local Node Configuration - Wireguard IPv4 [ %s ] IPv6 [ %s ]", ax.TunnelIP, ax.TunnelIpV6)
	// set the node unique local interface configuration
	ax.wgConfig.Interface = localInterface
}
