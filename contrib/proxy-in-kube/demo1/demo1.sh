#!/bin/bash

set -e

SERVICE=https://qa.nexodus.io
NAMESPACE=demo1
KUBECTL_APPLY="kubectl apply -f -"

function help() {
  echo "Usage: $0 [-u USERNAME] [-p PASSWORD] [-s SERVICE] [-h]"
  echo "  -u USERNAME   Set the username (required)"
  echo "  -p PASSWORD   Set the password (required)"
  echo "  -s SERVICE    Set the service URL (default: ${SERVICE_URL})"
  echo "  -n NAMESPACE  Set the kubernetes namespace (default: ${NAMESPACE})"
  echo "  -d            Dry run. Output yaml instead of applying it to the cluster."
  echo "  -h            Display this help message"
}

# Parse command-line options
while getopts "u:p:s:hd" opt; do
  case ${opt} in
    u ) USERNAME="$OPTARG";;
    p ) PASSWORD="$OPTARG";;
    s ) SERVICE="$OPTARG";;
    n ) NAMESPACE="$OPTARG";;
    d ) KUBECTL_APPLY="cat";;
    h ) help; exit 0;;
    \? ) echo "Invalid option: -$OPTARG" >&2; exit 1;;
    : ) echo "Option -$OPTARG requires an argument" >&2; exit 1;;
  esac
done

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
  echo "ERROR: Username and password are required."
  echo
  help
  exit 1
fi

if [ "${KUBECTL_APPLY}" != "cat" ]; then
    echo "Using Nexodus Service: ${SERVICE}"
    echo "Current context for kubectl: $(kubectl config current-context)"
fi

if [ ! -f public.key ] || [ ! -f private.key ]; then
    wg genkey | tee private.key | wg pubkey > public.key
fi

kubectl create namespace "${NAMESPACE}" -o yaml --dry-run=client | ${KUBECTL_APPLY}

# Create a secret with the username and password for the Nexodus Service
kubectl create secret generic nexodus-credentials --from-literal=username="${USERNAME}" --from-literal=password="${PASSWORD}" -n "${NAMESPACE}" -o yaml --dry-run=client | ${KUBECTL_APPLY}

# Create a secret that holes the public and private keys for wireguard
kubectl create secret generic wireguard-keys --from-literal=private.key="$(cat private.key)" --from-literal=public.key="$(cat public.key)" -n "${NAMESPACE}" -o yaml --dry-run=client | ${KUBECTL_APPLY} 
# Create an nginx deployment that responds with the name of the Pod the response
# is coming from.
cat nginx.yaml.in | sed -e 's/${NAMESPACE}/'"${NAMESPACE}/" | ${KUBECTL_APPLY}

# Deploy nexd proxy with a rule that exposes the nginx service.
cat demo1.yaml.in | sed -e 's/${NAMESPACE}/'"${NAMESPACE}/" | ${KUBECTL_APPLY}
