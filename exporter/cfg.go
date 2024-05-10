package exporter

import (
	"net/http"

	"github.com/NilFoundation/nil/core/types"
)

type Cfg struct {
	APIEndpoints   []string
	ExporterDriver ExportDriver
	httpClient     http.Client
	BlocksChan     chan []*types.Block
	ErrorChan      chan error
	used           bool
	FetchersCount  uint64
}

func (cfg *Cfg) pickAPIEndpoint() string {
	return cfg.APIEndpoints[0]
}
