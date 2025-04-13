package rollupcontract

import (
	"bytes"
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
func (r *wrapperImpl) UpdateState(
	ctx context.Context,
	batchIndex string,
	dataProofs types.DataProofs,
	oldStateRoot, newStateRoot common.Hash,
	validityProof []byte,
	publicDataInputs INilRollupPublicDataInfo,
) error {
	if oldStateRoot.Empty() {
		return errors.New("old state root is empty")
	}
	if newStateRoot.Empty() {
		return errors.New("new state root is empty")
	}

	batchState, err := r.getBatchState(ctx, batchIndex)
	if err != nil {
		return err
	}

	if batchState.IsFinalized {
		return fmt.Errorf("%w: batchId=%s", ErrBatchAlreadyFinalized, batchIndex)
	}

	if !batchState.IsCommitted {
		return fmt.Errorf("%w: batchId=%s", ErrBatchNotCommitted, batchIndex)
	}

	// Get last finalized batch index
	lastFinalizedBatchIndex, err := r.FinalizedBatchIndex(ctx)
	if err != nil {
		return err
	}

	// Get last finalized state root
	lastFinalizedstateRoot, err := r.rollupContract.FinalizedStateRoots(r.getEthCallOpts(ctx), lastFinalizedBatchIndex)
	if err != nil {
		return err
	}

	if !bytes.Equal(lastFinalizedstateRoot[:], oldStateRoot.Bytes()) {
		return fmt.Errorf("last finalized state root (%s) and oldStateRoot (%s) differ, batchId=%s",
			lastFinalizedstateRoot, oldStateRoot, batchIndex)
	}

	var tx *ethtypes.Transaction
	if err := r.transactWithCtx(ctx, func(opts *bind.TransactOpts) error {
		var err error
		tx, err = r.rollupContract.UpdateState(
			opts,
			batchIndex,
			oldStateRoot,
			newStateRoot,
			dataProofs,
			validityProof,
			publicDataInputs,
		)
		return err
	}); err != nil {
		return fmt.Errorf("UpdateState transaction failed: %w", err)
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
		return errors.New("UpdateState tx failed")
	}

	return err
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
