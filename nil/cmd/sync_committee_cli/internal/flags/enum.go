package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

type EnumFlag interface {
	pflag.Value
	PossibleValues() []string
}

func EnumVar[E EnumFlag](flagSet *pflag.FlagSet, value E, name string, usage string) {
	usageWithVals := fmt.Sprintf(
		"%s, possible values: %s",
		usage,
		strings.Join(value.PossibleValues(), ", "),
	)

	flagSet.Var(value, name, usageWithVals)
}
