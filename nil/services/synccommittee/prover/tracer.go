package prover

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type RemoteTracer interface {
	GetBlockTraces(ctx context.Context, shardId types.ShardId, blockRef transport.BlockReference) (ExecutionTraces, error)
}

type RemoteTracerImpl struct {
	client *rpc.Client
	logger zerolog.Logger
}

var _ RemoteTracer = new(RemoteTracerImpl)

func NewRemoteTracer(client *rpc.Client, logger zerolog.Logger) (*RemoteTracerImpl, error) {
	return &RemoteTracerImpl{
		client: client,
		logger: logger,
	}, nil
}

func (rt *RemoteTracerImpl) GetBlockTraces(ctx context.Context, shardId types.ShardId, blockRef transport.BlockReference) (ExecutionTraces, error) {
	dbgBlock, err := rt.client.GetDebugBlock(shardId, blockRef, true)
	if err != nil {
		return ExecutionTraces{}, err
	}
	if dbgBlock == nil {
		return ExecutionTraces{}, errors.New("client returned nil block")
	}

	decodedDbgBlock, err := dbgBlock.DecodeSSZ()
	if err != nil {
		return ExecutionTraces{}, err
	}

	if decodedDbgBlock.Id == 0 {
		// TODO: prove genesis block generation?
		return ExecutionTraces{}, nil
	}

	prevBlock, err := rt.client.GetBlock(shardId, transport.BlockNumber(decodedDbgBlock.Id-1), false)
	if err != nil {
		return ExecutionTraces{}, err
	}

	if prevBlock.MainChainHash == common.EmptyHash {
		// TODO: shard has just started, no reference to MainChain
		return ExecutionTraces{}, err
	}

	getHashFunc := func(blkNum uint64) (common.Hash, error) {
		// TODO: try to replace it with prevBlock.Hash
		_ = prevBlock.Hash
		block, err := rt.client.GetBlock(shardId, transport.BlockNumber(blkNum), false)
		if err != nil {
			return common.EmptyHash, err
		}
		return block.Hash, nil
	}

	blkContext := &vm.BlockContext{
		GetHash:     getHashFunc,
		BlockNumber: prevBlock.Number.Uint64(),
		Random:      &common.EmptyHash,
		BaseFee:     big.NewInt(10),
		BlobBaseFee: big.NewInt(10),
		Time:        decodedDbgBlock.Timestamp,
	}

	localDb, err := db.NewBadgerDbInMemory() // TODO: move this creation to caller
	if err != nil {
		return ExecutionTraces{}, err
	}

	stateDB, err := NewTracerStateDB(ctx, rt.client, shardId, prevBlock.Number, blkContext, localDb)
	if err != nil {
		return ExecutionTraces{}, err
	}

	stateDB.GasPrice = decodedDbgBlock.GasPrice
	for _, inMsg := range decodedDbgBlock.InMessages {
		_, msgHadErr := decodedDbgBlock.Errors[inMsg.Hash()]
		if msgHadErr {
			continue
		}

		if inMsg.Flags.GetBit(types.MessageFlagResponse) {
			panic("can't process responses in prover, refer to TryProcessResponse of ExecutionState")
		}

		stateDB.AddInMessage(inMsg)
		err := stateDB.HandleInMessage(inMsg)
		if err != nil {
			return ExecutionTraces{}, err
		}
	}

	// Print stats
	stats := stateDB.Stats
	rt.logger.Info().Msgf(
		"Tracer stats: processed %d inMessages out of %d with %d operations (stack %d, mem %d, store %d)",
		stats.ProcessedInMsgsN,
		len(decodedDbgBlock.InMessages),
		stats.OpsN,
		stats.StackOpsN,
		stats.MemoryOpsN,
		stats.StoreOpsN,
	)

	return stateDB.Traces, nil
}
