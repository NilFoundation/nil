package types

import (
	"testing"
)

func TestEmptyCode(t *testing.T) {
	c := Code(nil)
	c.Hash()
}
