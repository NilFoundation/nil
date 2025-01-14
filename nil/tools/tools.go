//go:build tools
// +build tools

package tools

import (
	_ "github.com/NilFoundation/fastssz/sszgen"
	_ "github.com/ethereum/go-ethereum/cmd/abigen"
	_ "github.com/matryer/moq"
)
