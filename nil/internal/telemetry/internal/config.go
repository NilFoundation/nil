package internal

type ExportOption int

const (
	ExportOptionNone ExportOption = iota
	ExportOptionStdout
	ExportOptionGrpc
)

type Config struct {
	ServiceName string

	TraceExportOption ExportOption
	TraceSamplingRate float64

	MetricExportOption ExportOption
}
