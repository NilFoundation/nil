package contracts

import "embed"

//go:generate bash -c "solc solidity/tests/*.sol --bin --abi --overwrite -o ./compiled/tests/"
//go:generate bash -c "solc solidity/*.sol --bin --abi --overwrite -o ./compiled/"
//go:embed compiled/*
var Fs embed.FS
