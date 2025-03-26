package rollupcontract

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// UpdateState attempts to update the state of a rollup contract using the provided proofs and state roots.
// It checks for non-empty state roots, validates the batch, verifies data proofs, and finally submits the update.
// Returns a nil on success or an error on validation failure or submission issues.
func (r *wrapperImpl) UpdateState(
	ctx context.Context,
	batchIndex string,
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

	commitTxSidecar, err := r.getCommitTransactionSidecar(ctx, batchIndex)
	if err != nil {
		return err
	}

	dataProofs, err := ComputeDataProofs(commitTxSidecar)
	if err != nil {
		return err
	}

	// to make sure proofs are correct before submission, not necessary, if other code is not buggy
	if err := r.verifyDataProofs(ctx, commitTxSidecar.BlobHashes(), dataProofs); err != nil {
		return err
	}

	var tx *ethtypes.Transaction
	if err := r.transactWithTimeout(ctx, func(opts *bind.TransactOpts) error {
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

func (r *wrapperImpl) verifyDataProofs(
	ctx context.Context,
	hashes []ethcommon.Hash,
	dataProofs [][]byte,
) error {
	for i, blobHash := range hashes {
		if err := r.rollupContract.VerifyDataProof(r.getEthCallOpts(ctx), blobHash, dataProofs[i]); err != nil {
			// TODO: make verification return a value.
			// Currently, no way to distinguish network error from verification one
			return fmt.Errorf("proof verification failed for blobHash=%s: %w", blobHash.Hex(), err)
		}
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

	if batchState.IsFinalized {
		return nil, fmt.Errorf("%w: batchId=%s", ErrBatchAlreadyFinalized, batchIndex)
	}

	// Check if batch is committed
	isCommitted, err := r.rollupContract.IsBatchCommitted(r.getEthCallOpts(ctx), batchIndex)
	if err != nil {
		return nil, err
	}
	batchState.IsCommitted = isCommitted

	if !batchState.IsCommitted {
		return nil, fmt.Errorf("%w: batchId=%s", ErrBatchNotCommitted, batchIndex)
	}

	return batchState, nil
}

func (r *wrapperImpl) getCommitTransactionSidecar(
	ctx context.Context,
	batchIndex string,
) (*ethtypes.BlobTxSidecar, error) {
	iter, err := r.rollupContract.FilterBatchCommitted(&bind.FilterOpts{Context: ctx}, []string{batchIndex})
	if err != nil {
		return nil, fmt.Errorf("FilterBatchCommitted failed: %w", err)
	}
	var commitTxHash *ethcommon.Hash
	for iter.Next() {
		// contract does not allow to commit the same batch ID multiple times, iter should contain only one entry
		check.PanicIfNot(commitTxHash == nil)
		commitTxHash = &iter.Event.Raw.TxHash
	}
	// Check for errors in case the iteration was stopped due to an error.
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("failed during event log iteration: %w", err)
	}
	// Close the iterator to release resources
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close the iterator: %w", err)
	}
	if commitTxHash == nil {
		return nil, fmt.Errorf("no commit transaction found for batch: %s", batchIndex)
	}

	tx, _, err := r.ethClient.TransactionByHash(ctx, *commitTxHash)
	if err != nil {
		return nil, fmt.Errorf("TransactionByHash failed: %w", err)
	}

	return tx.BlobTxSidecar(), nil
}
