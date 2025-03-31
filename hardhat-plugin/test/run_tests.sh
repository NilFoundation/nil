#!/usr/bin/env bash

set -e

NILD=$1
COMETA=$2
NIL=nil

trap_with_arg() {
    local func="$1"
    shift
    for sig in "$@"; do
        trap "$func $sig" "$sig"
    done
}

stop() {
    trap - SIGINT EXIT
    printf '\n%s\n' "received $1, killing child processes"
    kill -s SIGINT $(jobs -pr)
}

trap_with_arg 'stop' EXIT SIGINT SIGTERM SIGHUP

# Clean up after previous runs
rm -f config.ini cometa.config.yaml
rm -rf test.db

$COMETA create-config -c cometa.config.yaml
printf "use-badger: true\n" >>cometa.config.yaml

# Start nild in background (will be auto-killed on exit)
$NILD run --http-port 8529 --collator-tick-ms=100 --cometa-config cometa.config.yaml >nild.log 2>&1 &
sleep 2

export NIL_RPC_ENDPOINT=http://127.0.0.1:8529
export COMETA_ENDPOINT=http://127.0.0.1:8529
export FAUCET_ENDPOINT=http://127.0.0.1:8527
$NIL -c config.ini config set rpc_endpoint "$NIL_RPC_ENDPOINT"
$NIL -c config.ini config set cometa_endpoint "$COMETA_ENDPOINT"
$NIL -c config.ini config set faucet_endpoint "$FAUCET_ENDPOINT"

echo "Rpc endpoint    : $NIL_RPC_ENDPOINT"
echo "Cometa endpoint : $COMETA_ENDPOINT"

# Update to reflect the new directory structure
# Move to the directory where the script is located
cd $(dirname "$0")
npm test
