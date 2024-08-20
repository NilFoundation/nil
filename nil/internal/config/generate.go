package config

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path params.go -include ../../common/length.go,../../common/hash.go,../types/block.go,../types/address.go,../types/uint256.go --objs ParamValidators,ValidatorInfo,ParamGasPrice
