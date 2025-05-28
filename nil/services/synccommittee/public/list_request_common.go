package public

import "fmt"

const (
	DefaultLimit  = 20
	DebugMinLimit = 1
	DebugMaxLimit = 1000
)

type ListRequest struct {
	Limit int `json:"limit"`
}

func newListRequestCommon(limit *int) ListRequest {
	targetLimit := DefaultLimit
	if limit != nil {
		targetLimit = *limit
	}

	return ListRequest{
		Limit: targetLimit,
	}
}

func (r *ListRequest) Validate() error {
	if r.Limit < DebugMinLimit || r.Limit > DebugMaxLimit {
		return fmt.Errorf(
			"limit must be between %d and %d, actual is %d", DebugMinLimit, DebugMaxLimit, r.Limit)
	}

	return nil
}
