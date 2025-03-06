package cobrax

import (
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/spf13/pflag"
)

func AddLogLevelFlag(fset *pflag.FlagSet, dst *string) {
	AddCustomLogLevelFlag(fset, "log-level", "l", dst)
}

func AddCustomLogLevelFlag(fset *pflag.FlagSet, name, short string, dst *string) {
	if *dst == "" {
		*dst = "info"
	}
	fset.StringVarP(dst, name, short, *dst, "log level: trace|debug|info|warn|error|fatal|panic")
}

func AddPprofPortFlag(fset *pflag.FlagSet, dst *int) {
	if *dst == 0 {
		*dst = profiling.DefaultPort
	}
	fset.IntVar(dst, "pprof-port", *dst, "port to serve pprof profiling information")
}
