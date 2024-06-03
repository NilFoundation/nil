package keygen

import "errors"

var (
	errMultipleSelected = errors.New("cannot specify both --new and --fromHex flags")
	errNoSelected       = errors.New("either --new or --fromHex flag is required")
)

const (
	newFlag     = "new"
	fromHexFlag = "fromHex"
)

var params = &keygenParams{}

type keygenParams struct {
	newKey        bool
	hexPrivateKey string
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *keygenParams) initRawParams() error {
	if p.newKey && p.hexPrivateKey != "" {
		return errMultipleSelected
	}

	if !p.newKey && p.hexPrivateKey == "" {
		return errNoSelected
	}
	return nil
}
