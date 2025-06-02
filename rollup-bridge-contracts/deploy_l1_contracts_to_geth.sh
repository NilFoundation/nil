echo "Deploying L1 contracts to geth"

rm -rf deployments
npx hardhat run scripts/wallet/fund-wallet.ts
npx hardhat run scripts/deploy-and-wire.ts --network geth

echo "Fetching deployed bridge contract address"
l1_bridge_contract_addr=$(jq -r '.networks.geth.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy' deploy/config/l1-deployment-config.json)
echo "L1BridgeMessenger deployed to: $l1_bridge_contract_addr"

echo "Fetching deployed rollup contract address"
l1_rollup_contract_addr=$(jq -r '.networks.geth.nilRollup.nilRollupContracts.nilRollupProxy' deploy/config/l1-deployment-config.json)
echo "NilRollupProxy deployed to: $l1_rollup_contract_addr"
