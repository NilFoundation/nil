package tracer

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type RemoteTracer interface {
	GetBlockTraces(ctx context.Context, aggTraces ExecutionTraces, shardId types.ShardId, blockRef transport.BlockReference) error
}

type RemoteTracerImpl struct {
	client client.Client
	logger zerolog.Logger
}

var _ RemoteTracer = new(RemoteTracerImpl)

func NewRemoteTracer(client client.Client, logger zerolog.Logger) (*RemoteTracerImpl, error) {
	return &RemoteTracerImpl{
		client: client,
		logger: logger,
	}, nil
}

func (rt *RemoteTracerImpl) GetBlockTraces(
	ctx context.Context,
	aggTraces ExecutionTraces,
	shardId types.ShardId,
	blockRef transport.BlockReference,
) error {
	dbgBlock, err := rt.client.GetDebugBlock(ctx, shardId, blockRef, true)
	if err != nil {
		return err
	}
	if dbgBlock == nil {
		return errors.New("client returned nil block")
	}

	decodedDbgBlock, err := dbgBlock.DecodeSSZ()
	if err != nil {
		return err
	}

	if decodedDbgBlock.Id == 0 {
		// TODO: prove genesis block generation?
		return ErrCantProofGenesisBlock
	}
	prevBlock, err := rt.client.GetBlock(ctx, shardId, transport.BlockNumber(decodedDbgBlock.Id-1), false)
	if err != nil {
		return err
	}

	getHashFunc := func(blkNum uint64) (common.Hash, error) {
		// TODO: try to replace it with prevBlock.Hash
		_ = prevBlock.Hash

		block, err := rt.client.GetBlock(ctx, shardId, transport.BlockNumber(blkNum), false)
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
		return err
	}

	stateDB, err := NewTracerStateDB(ctx, aggTraces, rt.client, shardId, prevBlock.Number, blkContext, localDb, rt.logger)
	if err != nil {
		return err
	}

	stateDB.GasPrice = decodedDbgBlock.GasPrice
	for _, inMsg := range decodedDbgBlock.InMessages {
		_, msgHadErr := decodedDbgBlock.Errors[inMsg.Hash()]
		if msgHadErr {
			continue
		}

		if inMsg.Flags.GetBit(types.MessageFlagResponse) {
			return errors.New("can't process responses in prover, refer to TryProcessResponse of ExecutionState")
		}

		stateDB.AddInMessage(inMsg)
		if err := stateDB.HandleInMessage(inMsg); err != nil { //nolint:contextcheck
			return err
		}
	}

	err = stateDB.FinalizeTraces()
	if err != nil {
		return err
	}

	// Print stats
	stats := stateDB.Stats
	rt.logger.Info().
		Uint("processedInMessages", stats.ProcessedInMsgsN).
		Uint("totalInMessages", uint(len(decodedDbgBlock.InMessages))).
		Uint("operations", stats.OpsN).
		Uint("stackOperations", stats.StackOpsN).
		Uint("memoryOperations", stats.MemoryOpsN).
		Uint("stateOperations", stats.StateOpsN).
		Uint("copyOperations", stats.CopyOpsN).
		Uint("expOperations", stats.ExpOpsN).
		Uint("affectedContracts", stats.AffectedContractsN).
		Msg("Tracer stats")

	return nil
}
