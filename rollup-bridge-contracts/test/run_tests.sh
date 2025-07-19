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
    local jobs_list=$(jobs -pr)
    if [ -n "$jobs_list" ]; then
        kill -s SIGINT $jobs_list
    fi
}

trap_with_arg 'stop' EXIT SIGINT SIGTERM SIGHUP

export ANVIL_RPC_ENDPOINT=http://127.0.0.1:8545
export ANVIL_PRIVATE_KEY=

export GETH_RPC_ENDPOINT=http://127.0.0.1:8545
export GETH_PRIVATE_KEY=

export SEPOLIA_RPC_ENDPOINT=
export SEPOLIA_PRIVATE_KEY=

export NIL_RPC_ENDPOINT=http://127.0.0.1:8529
export FAUCET_ENDPOINT=http://127.0.0.1:8527
export NIL_PRIVATE_KEY=0x4d47e8aed46e8b1bb4f4573f68ad43cade273d149b0c2942526ad5141c51b517
export NIL=0x0001111111111111111111111111111111111110

echo "Rpc endpoint: $NIL_RPC_ENDPOINT"
echo "Private key: $NIL_PRIVATE_KEY"

# Update to reflect the new directory structure
# Move to the directory where the script is located
cd $(dirname "$0")

set +e
if CI=true npx hardhat test --network nil hardhat/*.ts; then
    exit 0
fi
