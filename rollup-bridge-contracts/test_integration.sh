#!/usr/bin/env bash

# Script to set up and run a local test environment for the Nil rollup bridge contracts
# Test plan:
# - Start a local geth node in dev mode
# - Deploy L1 contracts to the geth node
# - Start the nild node
# - Start faucet service
# - Start the relayer service
# - TODO: Deploy L2 contracts to the nild node
# - Trigger L1 deposit event
# - TODO: Ensure that the event was received and processed by the L2 side

set -euo pipefail

source .env

# Check if required environment variables are set
if [ -z "$GETH_BIN" ]; then
    echo "GETH_BIN is not set!"
    exit 1
fi

if [ -z "$NILD_BIN" ]; then
    echo "NILD_BIN is not set!"
    exit 1
fi

if [ -z "$FAUCET_BIN" ]; then
    echo "FAUCET_BIN is not set!"
    exit 1
fi

if [ -z "$RELAYER_BIN" ]; then
    echo "RELAYER_BIN is not set!"
    exit 1
fi

LOG_DIR=${LOG_DIR:-"."}
GETH_DATA_DIR=${GETH_DATA_DIR:-"."}

pids=()

cleanup() {
    echo "→ cleaning up ${#pids[@]} processes"
    cat $LOG_DIR/relayer.log

    for pid in "${pids[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" || true # polite TERM

            # rough SIGKILL
            timeout 5s bash -c "while kill -0 $pid 2>/dev/null; do sleep 0.1; done" ||
                kill -9 "$pid" || true
        fi
    done
}
trap cleanup EXIT INT TERM

wait_for_http_service() {
    local url="$1"
    local max_retries=10
    local delay=1

    for ((i = 0; i < max_retries; i++)); do
        if curl --silent --fail $url >/dev/null; then
            echo "Service is up: $url"
            return 0
        fi
        echo "Waiting for service to be up: $url"
        sleep "$delay"
    done

    echo "Service did not start in time: $url"
    return 1
}

echo "Preparing env for testing"

echo "Starting geth in dev mode"
mkdir -p $GETH_DATA_DIR
$GETH_BIN \
    --http.vhosts "'*,localhost,host.docker.internal'" \
    --http --http.api admin,debug,web3,eth,txpool,miner,net,dev \
    --http.corsdomain "*" --http.addr "0.0.0.0" --http.port 8545 --nodiscover \
    --maxpeers 0 --mine --networkid 1337 \
    --verbosity 7 \
    --datadir $GETH_DATA_DIR \
    --dev --dev.period 1 --allow-insecure-unlock --rpc.allow-unprotected-txs --dev.gaslimit 200000000 \
    >$LOG_DIR/geth.log 2>&1 &
pids+=("$!")
wait_for_http_service $GETH_RPC_ENDPOINT

echo "Deploying L1 contracts to geth"
rm -rf deployments
npx hardhat run scripts/wallet/fund-wallet.ts
npx hardhat run scripts/deploy-and-wire.ts --network geth

echo "Fetching deployed contract address"
l1_contract_addr=$(jq -r '.networks.geth.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy' deploy/config/l1-deployment-config.json)
echo "L1BridgeMessenger deployed to: $l1_contract_addr"

echo "Starting nild"
$NILD_BIN run --http-port 8529 --collator-tick-ms=100 >$LOG_DIR/nild.log 2>&1 &
pids+=("$!")
wait_for_http_service "http://127.0.0.1:8529"

echo "Starting faucet"
$FAUCET_BIN run --port 8527 &
pids+=("$!")
wait_for_http_service "http://127.0.0.1:8527"

npx hardhat l2-task-runner --networkname local --l1networkname geth

echo "Starting relayer"
$RELAYER_BIN run \
    --db-path=/tmp/relayer.db \
    --debug-rpc-endpoint=tcp://127.0.0.1:7777 \
    --l1-endpoint=$GETH_DATA_DIR/geth.ipc \
    --l1-contract-addr=$l1_contract_addr \
    --l2-endpoint=http://127.0.0.1:8529 \
    --l2-debug-mode=true \
    --l2-smart-account-salt=1234567890 \
    --l2-faucet-address=http://127.0.0.1:8527 \
    --l2-contract-addr=0xdeadbeef \
    >$LOG_DIR/relayer.log 2>&1 &
pids+=("$!")
wait_for_http_service "http://127.0.0.1:7777"

# example query to get relayer stats
curl -XPOST http://127.0.0.1:7777 -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"relayerDebug_getStats", "params": [], "id": 1}'

echo "TODO: L2 contract deployment is not ready yet"

echo "Triggering L1 deposit event"
npx hardhat run scripts/bridge-test/bridge-eth.ts --network geth

echo "TODO: ensure that event was received and processed by the L2 side"
# uncomment to wait till the block with the event is finalized
# sleep 60
