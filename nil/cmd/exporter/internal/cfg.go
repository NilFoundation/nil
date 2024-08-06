package internal

import (
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client"
)

type Cfg struct {
	ExporterDriver ExportDriver
	Client         client.Client
	BlocksChan     chan *BlockWithShardId
	exportRound    atomic.Uint32
}

func (cfg *Cfg) incrementRound() {
	cfg.exportRound.CompareAndSwap(100000, 0)
	cfg.exportRound.Add(1)
}
