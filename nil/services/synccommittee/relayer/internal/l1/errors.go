package l1

import "errors"

var ErrKeyExists = errors.New("object is already stored into the database")

func ignoreErrors(target error, toIgnore ...error) error {
	for _, err := range toIgnore {
		if errors.Is(target, err) {
			return nil
		}
	}
	return target
}
