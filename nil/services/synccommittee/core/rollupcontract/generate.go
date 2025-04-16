package rollupcontract

//go:generate go run github.com/ethereum/go-ethereum/cmd/abigen --abi=./abi.json --pkg=rollupcontract --out=./rollupcontract_abi_generated.go
//go:generate bash ../../internal/scripts/generate_mock.sh EthClient
