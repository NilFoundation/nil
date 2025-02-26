package config

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path params.go -include ../types/address.go,../types/uint256.go,../types/transaction.go,../../common/hash.go,../../common/length.go --objs ListValidators,ParamValidators,ValidatorInfo,ParamGasPrice,ParamFees,ParamL1BlockInfo,ParamSudoKey,WorkaroundToImportTypes
