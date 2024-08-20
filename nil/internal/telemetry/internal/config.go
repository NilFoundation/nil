package internal

type Config struct {
	ServiceName string `yaml:"serviceName"`

	ExportMetrics bool `yaml:"exportMetrics"`
}
