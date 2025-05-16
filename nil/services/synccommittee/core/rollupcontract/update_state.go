package rollupcontract

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// UpdateState attempts to update the state of a rollup contract using the provided proofs and state roots.
// It checks for non-empty state roots, validates the batch, verifies data proofs, and finally submits the update.
// Returns a nil on success or an error on validation failure or submission issues.
func (r *wrapperImpl) UpdateState(ctx context.Context, data *types.UpdateStateData) error {
	if err := r.validateUpdateStateData(data); err != nil {
		return fmt.Errorf("invalid update state data: %w", err)
	}

	batchIdStr := data.BatchId.String()
	batchState, err := r.getBatchState(ctx, batchIdStr)
	if err != nil {
		return err
	}
	if batchState.IsFinalized {
		return fmt.Errorf("%w: batchId=%s", ErrBatchAlreadyFinalized, data.BatchId)
	}
	if !batchState.IsCommitted {
		return fmt.Errorf("%w: batchId=%s", ErrBatchNotCommitted, data.BatchId)
	}

	latestFinalizedStateRoot, err := r.GetLatestFinalizedStateRoot(ctx)
	if err != nil {
		return err
	}
	if latestFinalizedStateRoot == common.EmptyHash {
		return types.ErrL1StateRootNotInitialized
	}

	if latestFinalizedStateRoot != data.OldProvedStateRoot {
		return fmt.Errorf("%w: latestFinalizedRoot=%s batchOldStateRoot=%s, batchId=%s",
			ErrOldStateRootMismatch, latestFinalizedStateRoot, data.OldProvedStateRoot, data.BatchId)
	}

	publicDataInputs := INilRollupPublicDataInfo{
		L2Tol1Root:    data.L2Tol1Root,
		L1MessageHash: data.L1MessageHash,
		DepositNonce:  data.DepositNonce,
	}

	// The transaction will be simulated (via eth_estimateGas) before submission,
	// but there is still a chance it may fail on-chain if the state changes
	// between simulation and actual inclusion in a block.
	var tx *ethtypes.Transaction
	if err := r.transactWithCtx(ctx, func(opts *bind.TransactOpts) error {
		var err error
		tx, err = r.rollupContract.UpdateState(
			opts,
			batchIdStr,
			data.OldProvedStateRoot,
			data.NewProvedStateRoot,
			data.DataProofs,
			data.ValidityProof,
			publicDataInputs,
		)
		return err
	}); err != nil {
		return fmt.Errorf("simulation transaction creation failed: %w", err)
	}

	r.logger.Info().
		Hex("txHash", tx.Hash().Bytes()).
		Int("gasLimit", int(tx.Gas())).
		Int("cost", int(tx.Cost().Uint64())).
		Msg("UpdateState transaction sent")

	receipt, err := r.waitForReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("error during waiting for receipt: %w", err)
	}
	r.logReceiptDetails(receipt)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		// Re-simulate the transaction on top of the block it originally failed in.
		// Note: The execution order of transactions in the block is not preserved during simulation,
		// so results may differ — but we attempt to identify the cause of failure anyway.
		err = r.simulateTx(ctx, tx, receipt.BlockNumber)
		if err != nil {
			return r.errorByName(fmt.Errorf("post-submition simulation: %w", err))
		}
		return errors.New("UpdateState tx failed, can't identify the reason")
	}

	return err
}

func (*wrapperImpl) validateUpdateStateData(data *types.UpdateStateData) error {
	// go-ethereum states not all RPC nodes support EVM errors parsing
	// explicitly check possible error in advance
	if data.OldProvedStateRoot.Empty() {
		return ErrInvalidOldStateRoot
	}
	if data.NewProvedStateRoot.Empty() {
		return ErrInvalidNewStateRoot
	}
	if len(data.ValidityProof) == 0 {
		return ErrInvalidValidityProof
	}
	if len(data.DataProofs) == 0 {
		return ErrEmptyDataProofs
	}
	return nil
}

// batchState contains validation results for a batch
type batchState struct {
	IsFinalized bool
	IsCommitted bool
}

func (r *wrapperImpl) getBatchState(
	ctx context.Context,
	batchIndex string,
) (*batchState, error) {
	batchState := &batchState{}

	// Check if batch is finalized
	isFinalized, err := r.rollupContract.IsBatchFinalized(r.getEthCallOpts(ctx), batchIndex)
	if err != nil {
		return nil, err
	}
	batchState.IsFinalized = isFinalized

	// Check if batch is committed
	isCommitted, err := r.rollupContract.IsBatchCommitted(r.getEthCallOpts(ctx), batchIndex)
	if err != nil {
		return nil, err
	}
	batchState.IsCommitted = isCommitted

	return batchState, nil
}
