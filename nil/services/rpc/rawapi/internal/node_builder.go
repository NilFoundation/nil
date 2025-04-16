package internal

import (
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type nodeApiBuilder struct {
	nodeApi *nodeApiOverShardApis

	// common dependencies
	db             db.ReadOnlyDB
	networkManager network.Manager
}

func NodeApiBuilder(db db.DB, networkManager network.Manager) *nodeApiBuilder {
	return &nodeApiBuilder{
		nodeApi: &nodeApiOverShardApis{
			apisRo:  make(map[types.ShardId]shardApiRo),
			apisRw:  make(map[types.ShardId]shardApiRw),
			apisDev: make(map[types.ShardId]shardApiDev),
			allApis: make([]shardApiBase, 0),
		},
		db:             db,
		networkManager: networkManager,
	}
}

func (nb *nodeApiBuilder) BuildAndReset() NodeApi {
	rv := *nb.nodeApi
	nb.nodeApi = &nodeApiOverShardApis{}
	for _, api := range rv.allApis {
		api.setNodeApi(&rv)
	}
	return &rv
}

func (nb *nodeApiBuilder) WithLocalShardApiRo(shardId types.ShardId) *nodeApiBuilder {
	var localShardApi shardApiRo = newLocalShardApiRo(shardId, nb.db)
	if assert.Enable {
		localShardApi = newShardApiClientDirectEmulatorRo(localShardApi)
	}
	nb.nodeApi.apisRo[shardId] = localShardApi
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, localShardApi)
	return nb
}

func (nb *nodeApiBuilder) WithLocalShardApiRw(shardId types.ShardId, txnpool txnpool.Pool) *nodeApiBuilder {
	var localShardApi shardApiRw = newLocalShardApiRw(newLocalShardApiRo(shardId, nb.db), txnpool)
	if assert.Enable {
		localShardApi = newShardApiClientDirectEmulatorRw(localShardApi)
	}
	nb.nodeApi.apisRw[shardId] = localShardApi
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, localShardApi)
	return nb
}

func (nb *nodeApiBuilder) WithNetworkShardApiClientRo(shardId types.ShardId) *nodeApiBuilder {
	networkShardApiClient := newShardApiClientNetworkRo(shardId, nb.networkManager)
	nb.nodeApi.apisRo[shardId] = networkShardApiClient
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, networkShardApiClient)
	return nb
}

func (nb *nodeApiBuilder) WithNetworkShardApiClientRw(shardId types.ShardId) *nodeApiBuilder {
	networkShardApiClient := newShardApiClientNetworkRw(shardId, nb.networkManager)
	nb.nodeApi.apisRw[shardId] = networkShardApiClient
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, networkShardApiClient)
	return nb
}

func (nb *nodeApiBuilder) WithLocalShardApiDev(shardId types.ShardId) *nodeApiBuilder {
	nb.nodeApi.apisDev[shardId] = newLocalShardApiDev(shardId)
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, nb.nodeApi.apisDev[shardId])
	return nb
}

func (nb *nodeApiBuilder) WithNetworkShardApiClientDev(shardId types.ShardId) *nodeApiBuilder {
	networkDevApiClient := newShardApiClientNetworkDev(shardId, nb.networkManager)
	nb.nodeApi.apisDev[shardId] = networkDevApiClient
	nb.nodeApi.allApis = append(nb.nodeApi.allApis, networkDevApiClient)
	return nb
}
