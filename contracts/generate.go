package contracts

import "embed"

//go:generate go run ../tools/solc/bin/main.go -s faucet.sol -o .
//go:embed *.bin
var Fs embed.FS
