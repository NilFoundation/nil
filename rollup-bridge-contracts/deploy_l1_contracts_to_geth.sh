echo "Deploying L1 contracts to geth"

set -e

rm -rf deployments
npx hardhat run scripts/wallet/fund-wallet.ts
npx hardhat run scripts/deploy-and-wire.ts --network geth

echo "Fetching deployed contracts addresses"

l1_contract_addr=$(jq -r '.networks.geth.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy' deploy/config/l1-deployment-config.json)
echo "L1BridgeMessenger deployed to: $l1_contract_addr"

l1_rollup_contract_address=$(jq -r '.networks.geth.nilRollup.nilRollupContracts.nilRollupProxy' deploy/config/l1-deployment-config.json)
echo "NilRollup deployed to: $l1_rollup_contract_address"

l1_gas_price_oracle_contract_address=$(jq -r '.networks.geth.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy' deploy/config/l1-deployment-config.json)
echo "GasPriceOracle deployed to: $l1_rollup_contract_address"
