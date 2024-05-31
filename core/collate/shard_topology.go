package collate

import (
	"fmt"

	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog/log"
)

type ShardTopology interface {
	GetNeighbours(id types.ShardId, nShards int) []types.ShardId
	ShouldPropagateMsg(from types.ShardId, to types.ShardId, dest types.ShardId) bool
}

type NeighbouringShardTopology struct{}

var neighbouringShardTopology ShardTopology = new(NeighbouringShardTopology)

func (_ *NeighbouringShardTopology) GetNeighbours(id types.ShardId, nShards int) []types.ShardId {
	var leftId, rightId types.ShardId
	if id > 0 {
		leftId = id - 1
	} else {
		leftId = types.ShardId(nShards - 1)
	}
	if id < types.ShardId(nShards)-1 {
		rightId = id + 1
	} else {
		rightId = 0
	}
	if leftId == rightId {
		return []types.ShardId{leftId}
	} else {
		return []types.ShardId{leftId, rightId}
	}
}

func (_ *NeighbouringShardTopology) ShouldPropagateMsg(from types.ShardId, to types.ShardId, dest types.ShardId) bool {
	// check that from->to and to->dest are in the same direction
	return (from < to) == (to < dest)
}

type TrivialShardTopology struct{}

var trivialShardTopology ShardTopology = new(TrivialShardTopology)

func (_ *TrivialShardTopology) GetNeighbours(id types.ShardId, nShards int) []types.ShardId {
	nb := make([]types.ShardId, 0, nShards)
	for i := range types.ShardId(nShards) {
		if i != id {
			nb = append(nb, i)
		}
	}
	return nb
}

func (_ *TrivialShardTopology) ShouldPropagateMsg(from types.ShardId, to types.ShardId, dest types.ShardId) bool {
	return false
}

const (
	TrivialShardTopologyId      = "TrivialShardTopology"
	NeighbouringShardTopologyId = "NeighbouringShardTopology"
)

func GetShardTopologyById(id string) ShardTopology {
	switch id {
	case TrivialShardTopologyId:
		return trivialShardTopology
	case NeighbouringShardTopologyId:
		return neighbouringShardTopology
	}
	err := fmt.Errorf("Uknown shard topology id: %v", id)
	log.Error().Err(err).Msgf("failed to get shard topology")
	panic(err)
}
