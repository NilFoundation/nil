package nildconfig

import (
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

type ReadThroughOptions struct {
	SourceAddr      string                `yaml:"sourceAddr"`
	ForkMainAtBlock transport.BlockNumber `yaml:"forkMainAtBlock"`
}

type Config struct {
	*nilservice.Config `yaml:",inline"`

	DB            *db.BadgerDBOptions `yaml:"db"`
	ReadThrough   *ReadThroughOptions `yaml:"readThrough,omitempty"`
	CometaConfig  string              `yaml:"cometaConfig,omitempty"`
	IndexerConfig string              `yaml:"indexerConfig,omitempty"`
}
