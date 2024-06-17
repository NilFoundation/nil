package contracts

import "embed"

//go:generate bash -c "solc solidity/tests/*.sol --bin --abi --overwrite -o ./compiled/tests/"
//go:generate solc solidity/Wallet.sol solidity/Faucet.sol --bin --abi --overwrite -o ./compiled
//go:embed compiled/*
var Fs embed.FS
