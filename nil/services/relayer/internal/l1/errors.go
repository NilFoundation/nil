package l1

import "errors"

var (
	ErrKeyExists            = errors.New("object is already stored into the database")
	ErrSerializationFailed  = errors.New("failed to (de)serialize object")
	ErrSubscriptionIsBroken = errors.New("l1 subscription is broken")
)

func ignoreErrors(target error, toIgnore ...error) error {
	for _, err := range toIgnore {
		if errors.Is(target, err) {
			return nil
		}
	}
	return target
}
