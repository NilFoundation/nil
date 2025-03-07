package indexer

import (
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	types2 "github.com/NilFoundation/nil/nil/services/indexer/types"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client"
)

type Cfg struct {
	IndexerDriver driver.IndexerDriver
	Client        client.Client
	AllowDbDrop   bool
	BlocksChan    chan *types2.BlockWithShardId
	indexRound    atomic.Uint32
}

func (cfg *Cfg) incrementRound() {
	cfg.indexRound.CompareAndSwap(100000, 0)
	cfg.indexRound.Add(1)
}
