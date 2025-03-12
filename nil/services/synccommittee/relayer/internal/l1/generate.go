package l1

// TODO(oclaw) do not copypaste ABI file, use one generated from actual L1BridgeMessenger.sol compilation
//go:generate go run github.com/ethereum/go-ethereum/cmd/abigen --abi=l1_bridge_messenger_abi.json --pkg=l1 --out=./l1_bridge_messenger_contract_abi_generated.go
//go:generate bash ../../../internal/scripts/generate_mock.sh EthClient
//go:generate bash ../../../internal/scripts/generate_mock.sh L1Contract
