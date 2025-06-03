#!/usr/bin/env bash

# Script to set up and run a local test environment for the Nil rollup bridge contracts
# Test plan:
# - Start a local geth node in dev mode
# - Deploy L1 contracts to the geth node
# - Start the nild node
# - Start faucet service
# - Deploy L2 contracts to the nild node
# - Set counterparty addresses in the L1 and L2 bridges
# - Start the relayer service
# - Grant relayer role to the relayer nil smart account
# - Trigger L1 deposit event
# - Ensure that the event was received and processed by the L2 side

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

if [ -z "$RELAYER_BIN" ]; then
    echo "RELAYER_BIN is not set!"
    exit 1
fi

if [ -z "$FAUCET_BIN" ]; then
    echo "FAUCET_BIN is not set!"
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
NO_CLEANUP=${NO_CLEANUP:-0}

if [[ NO_CLEANUP -ne 1 ]]; then
    trap cleanup EXIT INT TERM
else
    echo "No cleanup will be performed after the script exits."
    trap EXIT INT TERM
fi

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

source deploy_l1_contracts_to_geth.sh

echo "Starting nild"
$NILD_BIN run --http-port 8529 --collator-tick-ms=100 --log-level=trace >$LOG_DIR/nild.log 2>&1 &
pids+=("$!")

nild_endpoint="http://127.0.0.1:8529"
wait_for_http_service $nild_endpoint

faucet_port=8527
faucet_endpoint="http://127.0.0.1:$faucet_port"

echo "Starting faucet service"
$FAUCET_BIN run \
    --port=$faucet_port \
    --node-endpoint=$nild_endpoint \
    >$LOG_DIR/faucet.log 2>&1 &
pids+=("$!")
wait_for_http_service $faucet_endpoint
echo "Faucet service is up"

source deploy_l2_contracts_to_nild.sh

echo "Starting relayer"
$RELAYER_BIN run \
    --db-path=/tmp/relayer.db \
    --debug-rpc-endpoint=tcp://127.0.0.1:7777 \
    --l1-endpoint=$GETH_DATA_DIR/geth.ipc \
    --l1-contract-addr=$l1_contract_addr \
    --l2-endpoint=$nild_endpoint \
    --l2-debug-mode=true \
    --l2-smart-account-salt=1234567890 \
    --l2-faucet-address=$faucet_endpoint \
    --l2-contract-addr=$l2_contract_addr \
    --l2-bridges-addresses=$l2_eth_bridge_addr,$l2_enshrined_token_bridge_addr \
    >$LOG_DIR/relayer.log 2>&1 &
pids+=("$!")
wait_for_http_service "http://127.0.0.1:7777"

echo "Triggering L1 deposit event"

npx hardhat grant-relayer-role --networkname local
npx hardhat run scripts/bridge-test/bridge-eth.ts --network geth

echo "Waiting for relayer to process L1 deposit event"
npx hardhat validate-l2-eth-bridging --networkname local

echo "Bridge test completed successfully"
