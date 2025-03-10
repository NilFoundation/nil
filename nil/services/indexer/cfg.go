package indexer

import (
	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
)

type Cfg struct {
	IndexerDriver driver.IndexerDriver
	Client        client.Client
	BlocksChan    chan *driver.BlockWithShardId
	AllowDbDrop   bool
}
