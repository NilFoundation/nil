#!/usr/bin/env bash

set -e

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
rm -f config.ini
rm -rf test.db

# Start nild in background (will be auto-killed on exit)
nild run >nild.log 2>&1 &
sleep 2

export NIL_RPC_ENDPOINT=http://127.0.0.1:8529
nil -c config.ini config set rpc_endpoint "$NIL_RPC_ENDPOINT"
export PRIVATE_KEY=$(nil -c config.ini keygen new -q)
export WALLET_ADDR=$(nil -c config.ini wallet new -q)

echo "Rpc endpoint: $NIL_RPC_ENDPOINT"
echo "Private key: $PRIVATE_KEY"
echo "Wallet addr: $WALLET_ADDR"

# Update to reflect the new directory structure
# Move to the directory where the script is located
cd $(dirname "$0")
CI=true npx hardhat test --network nil test/*.ts
