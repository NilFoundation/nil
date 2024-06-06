package contracts

import "embed"

//go:generate sh ./generate.sh compiled
//go:embed compiled/*.bin
var Fs embed.FS
