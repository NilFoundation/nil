package cmdflags

import (
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/spf13/pflag"
)

func AddTelemetry(fset *pflag.FlagSet, config *telemetry.Config) {
	fset.BoolVar(&config.ExportMetrics, "metrics", config.ExportMetrics, "export metrics via grpc")
	fset.IntVar(
		&config.PrometheusPort,
		"prometheus-port",
		config.PrometheusPort,
		"port to serve prometheus metrics; 0 to disable")
}
