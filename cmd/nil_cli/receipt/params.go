package receipt

import "errors"

var errNoSelected = errors.New("at least one flag (--hash) is required")

const (
	hashFlag = "hash"
)

var params = &receiptParams{}

type receiptParams struct {
	hash string
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *receiptParams) initRawParams() error {
	flagsSet := 0

	if p.hash != "" {
		flagsSet++
	}

	if flagsSet == 0 {
		return errNoSelected
	}

	return nil
}
