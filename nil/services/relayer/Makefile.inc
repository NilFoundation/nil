root_relayer = nil/services/relayer
root_contracts = rollup-bridge-contracts/contracts

.PHONY: $(root_relayer)/compile_abi
$(root_relayer)/compile_abi:
	solc $(root_contracts)/bridge/l1/interfaces/IRelayMessage.sol --abi --overwrite -o $(root_relayer)/rollup-bridge-contracts-compiled-abi/contracts/bridge/l1/L1BridgeMessenger.sol --allow-paths .,$(root_contracts)/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json
	solc $(root_contracts)/bridge/l2/interfaces/IRelayMessage.sol --abi --overwrite -o $(root_relayer)/rollup-bridge-contracts-compiled-abi/contracts/bridge/l2/L2BridgeMessenger.sol --allow-paths .,$(root_contracts)/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json

.PHONY: $(root_relayer)/embed_l2_abi
$(root_relayer)/embed_l2_abi: $(root_relayer)/compile_abi
	cp $(root_relayer)/rollup-bridge-contracts-compiled-abi/contracts/bridge/l2/L2BridgeMessenger.sol/IRelayMessage.abi $(root_relayer)/internal/l2/L2BridgeMessenger.json.abi

.PHONY: $(root_relayer)/embed_l1_abi
$(root_relayer)/embed_l1_abi: $(root_relayer)/compile_abi
	cp $(root_relayer)/rollup-bridge-contracts-compiled-abi/contracts/bridge/l1/L1BridgeMessenger.sol/IRelayMessage.abi $(root_relayer)/internal/l1/L1BridgeMessenger.json.abi

.PHONY: $(root_relayer)/generate_l1_abi
$(root_relayer)/generate_l1_abi: $(root_relayer)/embed_l1_abi
	cd $(root_relayer)/internal/l1 && go run github.com/ethereum/go-ethereum/cmd/abigen --abi L1BridgeMessenger.json.abi --pkg=l1 --out=./l1_bridge_messenger_contract_abi_generated.go

.PHONY: $(root_relayer)/gen_l1_mocks
$(root_relayer)/gen_l1_mocks: $(root_relayer)/generate_l1_abi
	cd $(root_relayer)/internal/l1 && mockery --name=EthClient
	cd $(root_relayer)/internal/l1 && mockery --name=L1Contract

.PHONY: $(root_relayer)/gen_l2_mocks
$(root_relayer)/gen_l2_mocks: $(root_relayer)/embed_l2_abi
	cd $(root_relayer)/internal/l2 && mockery --name=L2Contract
