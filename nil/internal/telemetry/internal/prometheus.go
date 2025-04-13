package internal

import (
	"net/http"
	"strconv"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartPrometheusServer(config *Config) {
	if config.PrometheusPort == 0 {
		return
	}

	prometheus.DefaultRegisterer = prometheus.WrapRegistererWith(prometheus.Labels{
		telattr.ServiceNameKey: config.GetServiceName(),
	}, prometheus.DefaultRegisterer)

	server := http.NewServeMux()
	server.Handle("/metrics", promhttp.Handler())
	go func() {
		check.PanicIfErr(http.ListenAndServe(":"+strconv.Itoa(config.PrometheusPort), server)) //nolint:gosec
	}()
}
