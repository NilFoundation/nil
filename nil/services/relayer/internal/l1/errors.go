package l1

import "errors"

var ErrSubscriptionIsBroken = errors.New("l1 subscription is broken")

func ignoreErrors(target error, toIgnore ...error) error {
	for _, err := range toIgnore {
		if errors.Is(target, err) {
			return nil
		}
	}
	return target
}
