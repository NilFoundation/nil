package internal

import "os"

type Config struct {
	ServiceName string `yaml:"serviceName,omitempty"`

	ExportMetrics bool   `yaml:"exportMetrics,omitempty"`
	GrpcEndpoint  string `yaml:"grpcEndpoint,omitempty"`

	PrometheusPort int `yaml:"prometheusPort,omitempty"`
}

func (c *Config) GetServiceName() string {
	if c.ServiceName != "" {
		return c.ServiceName
	}

	// https://opentelemetry.io/docs/languages/sdk-configuration/general/#otel_service_name
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}

	return os.Args[0]
}
