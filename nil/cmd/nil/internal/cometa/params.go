package cometa

import "github.com/NilFoundation/nil/nil/internal/types"

var params = &cometaParams{}

type cometaParams struct {
	address       types.Address
	saveToFile    string
	inputJsonFile string
}
