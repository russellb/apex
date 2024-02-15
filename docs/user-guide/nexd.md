# Nexodus Node Agent `nexd`

## Overview

`nexd` implements a node agent to configure encrypted mesh networking on your device with nexodus.

<!--  everything after this comment is generated with: ./hack/nexd-docs.sh -->
### Usage

```text
NAME:
   nexd - Node agent to configure encrypted mesh networking with nexodus.

USAGE:
   nexd [global options] [command [command options]] [arguments...]

COMMANDS:
   version    Get the version of nexd
   proxy      Run nexd as an L4 proxy instead of creating a network interface
   router     Enable advertise-cidr function of the node agent to enable prefix forwarding.
   relay      Enable relay and discovery support function for the node agent.
   relayderp  Enable DERP relay to relay traffic between nexd nodes.
   help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --exit-node-client         Enable this node to use an available exit node (default: false) [$NEXD_EXIT_NODE_CLIENT]
   --help, -h                 Show help (default: false)
   --security-group-id value  Optional security group ID to use when registering used to secure this device [$NEXAPI_SECURITY_GROUP_ID]
   --unix-socket value        Path to the unix socket nexd is listening against (default: /var/run/nexd.sock)

   Agent Options

   --relay-only  Set if this node is unable to NAT hole punch or you do not want to fully mesh (Nexodus will set this automatically if symmetric NAT is detected) (default: false) [$NEXD_RELAY_ONLY]

   Nexodus Service Options

   --insecure-skip-tls-verify                   If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure (default: false) [$NEXD_INSECURE_SKIP_TLS_VERIFY]
   --password string                            Password string for accessing the nexodus service [$NEXD_PASSWORD]
   --service-url value                          URL to the Nexodus service (default: "https://try.nexodus.127.0.0.1.nip.io") [$NEXD_SERVICE_URL]
   --state-dir value                            Directory to store state in, such as api tokens to reuse after interactive login. (default: $HOME/.nexodus) [$NEXD_STATE_DIR]
   --stun-server value [ --stun-server value ]  stun server to use discover our endpoint address.  At least two are required. [$NEXD_STUN_SERVER]
   --username string                            Username string for accessing the nexodus service [$NEXD_USERNAME]
   --vpc-id value                               VPC ID to use when registering with the nexodus service [$NEXD_VPC_ID]

   Wireguard Options

   --listen-port port      Wireguard port to listen on for incoming peers (default: 0) [$NEXD_LISTEN_PORT]
   --local-endpoint-ip IP  Specify the endpoint IP address of this node instead of being discovered (optional) [$NEXD_LOCAL_ENDPOINT_IP]
   --request-ip IPv4       Request a specific IPv4 address from IPAM if available (optional) [$NEXD_REQUESTED_IP]

```

#### nexd proxy

```text
NAME:
   nexd proxy - Run nexd as an L4 proxy instead of creating a network interface

USAGE:
   nexd proxy [command [command options]] 

OPTIONS:
   --ingress value [ --ingress value ]  Forward connections from the Nexodus network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via a locally accessible network using a value in the form: protocol:port:destination_ip:destination_port. All fields are required.
   --egress value [ --egress value ]    Forward connections from a locally accessible network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via the Nexodus network using a value in the form: protocol:port:destination_ip:destination_port. All fields are required.
   --run-test-service                   Run a test service within the proxy instance to provide a quick and easy network endpoint on a nexodus network for testing purposes. You can connect to the proxy's nexodus IP on port 80 via http. This option can not be combined with an --ingress proxy rule listening on port 80. (default: false)
   --help, -h                           Show help (default: false)
```

#### nexd router

```text
NAME:
   nexd router - Enable advertise-cidr function of the node agent to enable prefix forwarding.

USAGE:
   nexd router [command [command options]] 

OPTIONS:
   --advertise-cidr CIDR [ --advertise-cidr CIDR ]  Request a CIDR range of addresses that will be advertised from this node (optional) [$NEXD_REQUESTED_ADVERTISE_CIDR]
   --network-router                                 Make the node a network router node that will forward traffic specified by --advertise-cidr through the physical interface that contains the default gateway (default: false) [$NEXD_NET_ROUTER_NODE]
   --disable-nat                                    disable NAT for the network router mode. This will require devices on the network to be configured with an ip route (default: false) [$NEXD_DISABLE_NAT]
   --exit-node                                      Enable this node to be an exit node. This allows other agents to source all traffic leaving the Nexodus mesh from this node (default: false) [$NEXD_EXIT_NODE]
   --help, -h                                       Show help (default: false)
```

#### nexd relay

```text
NAME:
   nexd relay - Enable relay and discovery support function for the node agent.

USAGE:
   nexd relay [command [command options]] 

OPTIONS:
   --help, -h  Show help (default: false)
```

#### nexd relayderp

```text
NAME:
   nexd relayderp - Enable DERP relay to relay traffic between nexd nodes.

USAGE:
   nexd relayderp [command [command options]] 

OPTIONS:
   --onboard                        Onboard the derp relay to nexodus and connect to local mesh network. (default: false) [$NEXD_DERP_ONBOARD]
   --addr value                     Server HTTP/HTTPS listen address, in form ":port", "ip:port", or for IPv6 "[ip]:port". (default: ":443") [$NEXD_DERP_LISTEN_ADDR]
   --stun-port value                The UDP port on which to serve STUN. (default: 3478) [$NEXD_DERP_STUN_PORT]
   --certmode value                 Mode for getting a cert. possible options: manual, letsencrypt (default: "letsencrypt") [$NEXD_DERP_CERT_MODE]
   --certdir value                  Directory to store LetsEncrypt certs. (default: $HOME/.nexodus) [$NEXD_DERP_CERT_DIR]
   --hostname value                 LetsEncrypt host name, if addr's port is :443 (default: "relay.nexodus.io") [$NEXD_DERP_HOSTNAME]
   --stun                           Run a STUN server. (default: true) [$NEXD_DERP_RUN_STUN]
   --accept-connection-limit value  Rate limit for accepting new connection (default: +Inf) [$NEXD_DERP_ACCEPT_CONN_LIMIT]
   --accept-connection-burst value  Burst limit for accepting new connection. (default: 9223372036854775807) [$NEXD_DERP_ACCEPT_CONN_BURST]
   --help, -h                       Show help (default: false)
```
