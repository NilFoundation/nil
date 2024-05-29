package exporter

import (
	"net/http"
	"sync/atomic"
)

type Cfg struct {
	APIEndpoints   []string
	ExporterDriver ExportDriver
	httpClient     http.Client
	BlocksChan     chan *BlockMsg
	ErrorChan      chan error
	used           bool
	exportRound    atomic.Uint32
}

func (cfg *Cfg) pickAPIEndpoint() string {
	return cfg.APIEndpoints[0]
}

func (cfg *Cfg) incrementRound() {
	cfg.exportRound.CompareAndSwap(100000, 0)
	cfg.exportRound.Add(1)
}
