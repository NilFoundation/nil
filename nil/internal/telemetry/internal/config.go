package internal

type ExportOption int

const (
	ExportOptionNone ExportOption = iota
	ExportOptionGrpc
)

type Config struct {
	ServiceName string

	MetricExportOption ExportOption
}
