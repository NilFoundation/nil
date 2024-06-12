package contracts

import "embed"

//go:generate solc wallet.sol --bin --abi --overwrite -o ./compiled
//go:generate sh ./generate.sh compiled
//go:embed compiled/*.*
var Fs embed.FS
