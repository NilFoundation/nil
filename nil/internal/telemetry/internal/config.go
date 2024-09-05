package internal

type Config struct {
	ServiceName string `yaml:"serviceName,omitempty"`

	ExportMetrics bool `yaml:"exportMetrics,omitempty"`
}
