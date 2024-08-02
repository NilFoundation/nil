package common

import (
	"fmt"
	"strings"
)

func Join[T fmt.Stringer](sep string, elems ...T) string {
	var sb strings.Builder
	for i, elem := range elems {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(elem.String())
	}
	return sb.String()
}
