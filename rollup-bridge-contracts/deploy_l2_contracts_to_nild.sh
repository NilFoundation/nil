echo "Deploying L2 contracts to nild"

set -e

npx hardhat l2-task-runner --networkname local --l1networkname geth
l2_contract_addr=$(jq -r '.networks.local.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy' deploy/config/nil-deployment-config.json)
l2_eth_bridge_addr=$(jq -r '.networks.local.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy' deploy/config/nil-deployment-config.json)
l2_enshrined_token_bridge_addr=$(jq -r '.networks.local.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy' deploy/config/nil-deployment-config.json)

echo "L2BridgeMessenger deployed to: $l2_contract_addr"
echo "L2ETHBridge deployed to: $l2_eth_bridge_addr"
echo "L2EnshrinedTokenBridge deployed to: $l2_enshrined_token_bridge_addr"

npx hardhat run scripts/wiring/bridges/l1/set-counterparty-in-bridges.ts --network geth
pnpm hardhat run scripts/wiring/wiring-master.ts --network geth
