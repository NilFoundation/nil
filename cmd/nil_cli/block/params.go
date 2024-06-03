package block

import "errors"

var (
	errNoSelected       = errors.New("at least one flag (--latest, --number, or --hash) is required")
	errMultipleSelected = errors.New("only one flag (--latest, --number, or --hash) can be set")
)

const (
	latestFlag = "latest"
	numberFlag = "number"
	hashFlag   = "hash"
)

var params = &blockParams{}

type blockParams struct {
	latest bool
	number string
	hash   string
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *blockParams) initRawParams() error {
	flagsSet := 0

	if p.latest {
		flagsSet++
	}
	if p.number != "" {
		flagsSet++
	}
	if p.hash != "" {
		flagsSet++
	}

	if flagsSet == 0 {
		return errNoSelected
	}
	if flagsSet > 1 {
		return errMultipleSelected
	}

	return nil
}
