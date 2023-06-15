package nexodus

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	iterations = 1
	interval   = 800
	timeWait   = 1200
)

const (
	protocolICMP     = 1
	protocolIPv6ICMP = 58
)

const (
	icmpTypeEchoRequest = 8
)

type KeepaliveStatus struct {
	WgIP        string `json:"wg_ip"`
	IsReachable bool   `json:"is_reachable"`
	Hostname    string `json:"hostname"`
}

func (nx *Nexodus) runProbe(peerStatus KeepaliveStatus, c chan struct {
	KeepaliveStatus
	IsReachable bool
}) {
	err := nx.ping(peerStatus.WgIP)
	if err != nil {
		nx.logger.Debugf("probe error: %v", err)
		// peer is not replying
		c <- struct {
			KeepaliveStatus
			IsReachable bool
		}{peerStatus, false}
	} else {
		// peer is replying
		c <- struct {
			KeepaliveStatus
			IsReachable bool
		}{peerStatus, true}
	}
}

func (nx *Nexodus) ping(host string) error {
	interval := time.Duration(interval)
	waitFor := time.Duration(timeWait) * time.Millisecond
	for i := uint64(0); i <= iterations; i++ {
		_, err := nx.doPing(host, i+1, waitFor)
		if err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}
		if iterations > 1 {
			time.Sleep(time.Millisecond * interval)
		}
	}
	return nil
}

func (nx *Nexodus) doPing(host string, i uint64, waitFor time.Duration) (string, error) {
	var networkType string
	var icmpType icmp.Type
	var icmpProto int

	if util.IsIPv6Address(host) {
		if nx.userspaceMode {
			networkType = "ping6"
		} else {
			networkType = "ip6:ipv6-icmp"
		}
		icmpType = ipv6.ICMPTypeEchoRequest
		icmpProto = protocolIPv6ICMP
	} else {
		if nx.userspaceMode {
			networkType = "ping4"
		} else {
			networkType = "ip4:icmp"
		}
		icmpType = ipv4.ICMPTypeEcho
		icmpProto = protocolICMP
	}

	var socket net.Conn
	var err error
	if nx.userspaceMode {
		socket, err = nx.userspaceNet.Dial(networkType, host)
	} else {
		socket, err = net.Dial(networkType, host)
	}
	if err != nil {
		return "", err
	}
	requestPing := icmp.Echo{
		Seq:  int(i),
		Data: []byte("pingity ping"),
	}
	icmpBytes, _ := (&icmp.Message{Type: icmpType, Code: icmpTypeEchoRequest, Body: &requestPing}).Marshal(nil)
	err = socket.SetReadDeadline(time.Now().Add(waitFor))
	if err != nil {
		return "", err
	}
	start := time.Now()
	_, err = socket.Write(icmpBytes)
	if err != nil {
		return "", err
	}
	n, err := socket.Read(icmpBytes[:])
	if err != nil {
		return "", err
	}
	replyPacket, err := icmp.ParseMessage(icmpProto, icmpBytes[:n])
	if err != nil {
		return "", err
	}
	nx.logger.Debugf("ping reply from %s with %d bytes: %v, replyPacket: %v", host, n, icmpBytes[:n], replyPacket)
	replyPing, ok := replyPacket.Body.(*icmp.Echo)
	if !ok {
		return "", fmt.Errorf("invalid reply type, message protocol: %d type: %d chksum: %d",
			replyPacket.Type.Protocol(), replyPacket.Type, replyPacket.Checksum)
	}
	if !bytes.Equal(replyPing.Data, requestPing.Data) || replyPing.Seq != requestPing.Seq {
		return "", fmt.Errorf("invalid ping reply: %v", replyPing)
	}
	resp := fmt.Sprintf("%d bytes from %v: icmp_seq=%v, time=%v", n, host, i, time.Since(start))
	nx.logger.Debug(resp)
	return resp, nil
}
