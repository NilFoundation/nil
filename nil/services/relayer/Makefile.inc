root_services = nil/services
root_relayer = nil/services/relayer

.PHONY: $(root_relayer)/embed_l2_abi
$(root_relayer)/embed_l2_abi:
	@cat $(root_services)/rollup-bridge-contracts-compiled/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json | jq .abi > $(root_relayer)/internal/l2/L2BridgeMessenger.json.abi

.PHONY: $(root_relayer)/embed_l1_abi
$(root_relayer)/embed_l1_abi:
	@cat $(root_services)/rollup-bridge-contracts-compiled/contracts/bridge/l1/L1BridgeMessenger.sol/L1BridgeMessenger.json | jq .abi > $(root_relayer)/internal/l1/L1BridgeMessenger.json.abi

.PHONY: $(root_relayer)/generate_l1_abi
$(root_relayer)/generate_l1_abi: $(root_relayer)/embed_l1_abi
	cd $(root_relayer)/internal/l1 && go run github.com/ethereum/go-ethereum/cmd/abigen --abi L1BridgeMessenger.json.abi --pkg=l1 --out=./l1_bridge_messenger_contract_abi_generated.go

.PHONY: $(root_relayer)/gen_l1_mocks
$(root_relayer)/gen_l1_mocks: $(root_relayer)/generate_l1_abi
	cd $(root_relayer)/internal/l1 && go run github.com/matryer/moq -out eth_client_generated_mock.go -rm -stub -with-resets . EthClient
	cd $(root_relayer)/internal/l1 && go run github.com/matryer/moq -out l1_contract_generated_mock.go -rm -stub -with-resets . L1Contract

.PHONY: $(root_relayer)/gen_l2_mocks
$(root_relayer)/gen_l2_mocks: $(root_relayer)/embed_l2_abi
	cd $(root_relayer)/internal/l2 && go run github.com/matryer/moq -out l2_contract_generated_mock.go -rm -stub -with-resets . L2Contract
