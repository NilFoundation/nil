package public

import "fmt"

const (
	DefaultLimit  = 20
	DebugMinLimit = 1
	DebugMaxLimit = 1000
)

type listRequestCommon struct {
	Limit int `json:"limit"`
}

func newListRequestCommon(limit *int) listRequestCommon {
	targetLimit := DefaultLimit
	if limit != nil {
		targetLimit = *limit
	}

	return listRequestCommon{
		Limit: targetLimit,
	}
}

func (r *listRequestCommon) Validate() error {
	if r.Limit < DebugMinLimit || r.Limit > DebugMaxLimit {
		return fmt.Errorf(
			"limit must be between %d and %d, actual is %d", DebugMinLimit, DebugMaxLimit, r.Limit)
	}

	return nil
}
