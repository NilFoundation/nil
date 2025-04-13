package l1

import "errors"

var (
	ErrSubscriptionIsBroken = errors.New("L1 subscription is broken")
	ErrInvalidEvent         = errors.New("invalid event from L1")
)

func ignoreErrors(target error, toIgnore ...error) error {
	for _, err := range toIgnore {
		if errors.Is(target, err) {
			return nil
		}
	}
	return target
}
