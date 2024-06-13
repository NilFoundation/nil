package contracts

import "embed"

//go:generate solc Wallet.sol --bin --abi --overwrite -o ./compiled
//go:generate sh ./generate.sh compiled
//go:embed compiled/*.*
var Fs embed.FS
