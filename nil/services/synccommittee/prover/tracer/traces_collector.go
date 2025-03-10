package tracer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
	"github.com/rs/zerolog"
)

type RemoteTracesCollector interface {
	GetBlockTraces(ctx context.Context, shardId types.ShardId, blockRef transport.BlockReference) (ExecutionTraces, error)
}

type RemoteTracesCollectorImpl struct {
	client api.RpcClient
	logger zerolog.Logger
}

var _ RemoteTracesCollector = new(RemoteTracesCollectorImpl)

type TraceConfig struct {
	ShardID      types.ShardId
	BlockIDs     []transport.BlockReference
	BaseFileName string
	MarshalMode  MarshalMode
}

func NewRemoteTracesCollector(client api.RpcClient, logger zerolog.Logger) (*RemoteTracesCollectorImpl, error) {
	return &RemoteTracesCollectorImpl{
		client: client,
		logger: logger,
	}, nil
}

func (rt *RemoteTracesCollectorImpl) GetBlockTraces(
	ctx context.Context,
	shardId types.ShardId,
	blockRef transport.BlockReference,
) (ExecutionTraces, error) {
	rt.logger.Debug().Stringer("blockRef", blockRef).Stringer(logging.FieldShardId, shardId).Msg("collecting traces for block")
	dbgBlock, err := rt.client.GetDebugBlock(ctx, shardId, blockRef, true)
	if err != nil {
		return nil, err
	}
	if dbgBlock == nil {
		return nil, ErrClientReturnedNilBlock
	}
	decodedDbgBlock, err := dbgBlock.DecodeSSZ()
	if err != nil {
		return nil, err
	}
	if decodedDbgBlock.Id == 0 {
		// TODO: prove genesis block generation?
		return nil, ErrCantProofGenesisBlock
	}

	prevBlock, err := rt.client.GetDebugBlock(ctx, shardId, transport.BlockNumber(decodedDbgBlock.Id-1), true)
	if err != nil {
		return nil, err
	}
	if prevBlock == nil {
		return nil, ErrClientReturnedNilBlock
	}
	decodedPrevDbgBlock, err := prevBlock.DecodeSSZ()
	if err != nil {
		return nil, err
	}

	localDb, err := db.NewBadgerDbInMemory()
	if err != nil {
		return nil, err
	}

	rwTx, err := localDb.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}

	configMap, gasPrices, err := rt.getConfigForBlock(ctx, decodedDbgBlock.Block, shardId, rwTx)
	if err != nil {
		return nil, err
	}

	// ExecutionState tries to read it from DB, execution differs if not written in advance
	if err := db.WriteBlock(rwTx, shardId, decodedPrevDbgBlock.Hash(shardId), decodedPrevDbgBlock.Block); err != nil {
		return nil, err
	}

	configAccessor := config.NewConfigAccessorFromMap(configMap)

	es, err := execution.NewExecutionState(
		rwTx,
		shardId,
		execution.StateParams{
			Block:          decodedPrevDbgBlock.Block,
			ConfigAccessor: configAccessor,
		},
	)
	if err != nil {
		return nil, err
	}

	esTracer := NewTracer(es)

	mptTracer := mpttracer.New(rt.client, decodedPrevDbgBlock.Id, rwTx, shardId)
	mptTracer.SetRootHash(decodedPrevDbgBlock.SmartContractsRoot)
	es.ContractTree = mptTracer

	es.EvmTracingHook = esTracer.getTracingHook()

	blockGeneratorParams := execution.NewBlockGeneratorParams(shardId, uint32(len(gasPrices)))
	blockGeneratorParams.EvmTracingHook = es.EvmTracingHook

	blockGenerator, err := execution.NewBlockGeneratorWithEs(
		ctx,
		blockGeneratorParams,
		localDb,
		rwTx,
		es,
	)
	if err != nil {
		return nil, err
	}

	proposal := execution.Proposal{
		PrevBlockId:   decodedPrevDbgBlock.Id,
		PrevBlockHash: decodedPrevDbgBlock.Hash(shardId),
		CollatorState: types.CollatorState{},
		MainChainHash: decodedDbgBlock.MainChainHash,
		ShardHashes:   decodedDbgBlock.ChildBlocks,

		InternalTxns: nil,
		ExternalTxns: nil,
		ForwardTxns:  nil,
	}
	proposal.InternalTxns, proposal.ExternalTxns = execution.SplitInTransactions(decodedDbgBlock.InTransactions)
	proposal.ForwardTxns, _ = execution.SplitOutTransactions(decodedDbgBlock.OutTransactions, shardId)
	rt.logger.Debug().Msg("building block")
	generatedBlock, err := blockGenerator.BuildBlock(&proposal, gasPrices)
	if err != nil {
		return nil, err
	}

	if generatedBlock.BlockHash != decodedDbgBlock.Hash(shardId) {
		return nil, fmt.Errorf("%w: fetched hash: %s, generated hash: %s", ErrTracedBlockHashMismatch, decodedDbgBlock.Hash(shardId), generatedBlock.BlockHash)
	}

	rt.logger.Debug().Msg("getting mpt traces")
	mptTraces, err := mptTracer.GetMPTTraces()
	if err != nil {
		return nil, err
	}
	esTracer.Traces.SetMptTraces(&mptTraces)

	return esTracer.Traces, nil
}

func GenerateTrace(ctx context.Context, rpcClient api.RpcClient, cfg *TraceConfig) error {
	remoteTracesCollector, err := NewRemoteTracesCollector(rpcClient, logging.NewLogger("tracer"))
	if err != nil {
		return err
	}
	aggregatedTraces := NewExecutionTraces()
	for _, blockID := range cfg.BlockIDs {
		traces, err := remoteTracesCollector.GetBlockTraces(ctx, cfg.ShardID, blockID)
		if err != nil {
			return err
		}
		if err := aggregatedTraces.Append(traces); err != nil {
			return err
		}
	}

	return SerializeToFile(aggregatedTraces, cfg.MarshalMode, cfg.BaseFileName)
}

func (rt *RemoteTracesCollectorImpl) getConfigForBlock(ctx context.Context, block *types.Block, shardId types.ShardId, rwTx db.RwTx) (map[string][]byte, []types.Uint256, error) {
	blockWithConfigHash := block.GetMainShardHash(shardId)
	blockWithConfig, err := rt.client.GetDebugBlock(ctx, types.MainShardId, blockWithConfigHash, true)
	if err != nil {
		return nil, nil, err
	}
	if blockWithConfig == nil {
		return nil, nil, ErrClientReturnedNilBlock
	}
	decodedBlockWithConfig, err := blockWithConfig.DecodeSSZ()
	if err != nil {
		return nil, nil, err
	}

	configMap, err := blockWithConfig.Config.ToMap()
	if err != nil {
		return nil, nil, err
	}

	// During config commit it creates trie ontop of DB, thus, we have to populate it
	configTrie := mpt.NewDbMPT(rwTx, types.MainShardId, db.ConfigTrieTable)
	for k, v := range configMap {
		if err := configTrie.Set([]byte(k), v); err != nil {
			return nil, nil, err
		}
	}

	gasPrices := []types.Uint256{}
	if shardId.IsMainShard() {
		// the main shard is omitted in ChildBlocks
		gasPrices = append(gasPrices, *decodedBlockWithConfig.BaseFee.Uint256)

		blockWithGasPricesId := decodedBlockWithConfig.Id
		if decodedBlockWithConfig.Id != 0 {
			blockWithGasPricesId--
		}
		gasPricesBlock, err := rt.client.GetDebugBlock(ctx, types.MainShardId, transport.BlockNumber(blockWithGasPricesId), true)
		if err != nil {
			return nil, nil, err
		}
		if gasPricesBlock == nil {
			return nil, nil, ErrClientReturnedNilBlock
		}
		decodedGasPricesBlock, err := gasPricesBlock.DecodeSSZ()
		if err != nil {
			return nil, nil, err
		}
		for i, blockHash := range decodedGasPricesBlock.ChildBlocks {
			childBlock, err := rt.client.GetBlock(ctx, types.ShardId(i), blockHash, false)
			if err != nil {
				return nil, nil, err
			}
			gasPrices = append(gasPrices, *childBlock.BaseFee.Uint256)
		}
	}

	return configMap, gasPrices, nil
}
