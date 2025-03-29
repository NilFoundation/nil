package rollupcontract

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type Wrapper struct {
	rollupContract  *Rollupcontract
	contractAddress ethcommon.Address
	requestTimeout  time.Duration
	privateKey      *ecdsa.PrivateKey
	chainID         *big.Int
	ethClient       EthClient
	logger          logging.Logger
}

func NewWrapper(
	ctx context.Context,
	contractAddressHex string,
	privateKeyHex string,
	ethClient EthClient,
	requestTimeout time.Duration,
	logger logging.Logger,
) (*Wrapper, error) {
	contactAddress := ethcommon.HexToAddress(contractAddressHex)
	rollupContract, err := NewRollupcontract(contactAddress, ethClient)
	if err != nil {
		return nil, fmt.Errorf("can't create rollup contract instance: %w", err)
	}

	privateKeyECDSA, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("converting private key hex to ECDSA: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chain ID: %w", err)
	}

	return &Wrapper{
		rollupContract:  rollupContract,
		contractAddress: contactAddress,
		requestTimeout:  requestTimeout,
		privateKey:      privateKeyECDSA,
		chainID:         chainID,
		ethClient:       ethClient,
		logger:          logger,
	}, nil
}

// UpdateState attempts to update the state of a rollup contract using the provided proofs and state roots.
// It checks for non-empty state roots, validates the batch, verifies data proofs, and finally submits the update.
// Returns a transaction pointer on success or an error on validation failure or submission issues.
func (r *Wrapper) UpdateState(
	ctx context.Context,
	batchIndex string,
	oldStateRoot, newStateRoot common.Hash,
	dataProofs [][]byte,
	blobHashes []ethcommon.Hash, // used for verification
	validityProof []byte,
	publicDataInputs INilRollupPublicDataInfo,
) (*ethtypes.Transaction, error) {
	if oldStateRoot.Empty() {
		return nil, errors.New("old state root is empty")
	}
	if newStateRoot.Empty() {
		return nil, errors.New("new state root is empty")
	}

	_, err := r.validateBatch(ctx, batchIndex, oldStateRoot)
	if err != nil {
		return nil, err
	}

	// to make sure proofs are correct before submission, not necessary
	if err := r.verifyDataProofs(ctx, blobHashes, dataProofs); err != nil {
		return nil, err
	}

	transactOpts, cancel, err := r.getEthTransactOpts(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return r.rollupContract.UpdateState(
		transactOpts,
		batchIndex,
		oldStateRoot,
		newStateRoot,
		dataProofs,
		validityProof,
		publicDataInputs,
	)
}

func (r *Wrapper) StateRoots(ctx context.Context, finalizedBatchIndex string) ([32]byte, error) {
	callOpts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	return r.rollupContract.FinalizedStateRoots(callOpts, finalizedBatchIndex)
}

func (r *Wrapper) FinalizedBatchIndex(ctx context.Context) (string, error) {
	callOpts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	return r.rollupContract.GetLastFinalizedBatchIndex(callOpts)
}

// WaitForReceipt repeatedly tries to get tx receipt, retrying on `NotFound` error (tx not mined yet).
// In case `ReceiptWaitFor` timeout is reached, returns `(nil, nil)`.
func (r *Wrapper) WaitForReceipt(ctx context.Context, txnHash ethcommon.Hash) (*ethtypes.Receipt, error) {
	const (
		ReceiptWaitFor  = 30 * time.Second
		ReceiptWaitTick = 500 * time.Millisecond
	)
	return common.WaitForValue(
		ctx,
		ReceiptWaitFor,
		ReceiptWaitTick,
		func(ctx context.Context) (*ethtypes.Receipt, error) {
			receipt, err := r.ethClient.TransactionReceipt(ctx, txnHash)
			if errors.Is(err, ethereum.NotFound) {
				// retry
				return nil, nil
			}
			return receipt, err
		})
}

func (r *Wrapper) verifyDataProofs(
	ctx context.Context,
	hashes []ethcommon.Hash,
	dataProofs [][]byte,
) error {
	opts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	for i, blobHash := range hashes {
		if err := r.rollupContract.VerifyDataProof(opts, blobHash, dataProofs[i]); err != nil {
			// TODO: make verification return a value.
			//  Currently, no way to distinguish network error from verification one
			return fmt.Errorf("proof verification failed for versioned hash %s (%w)", blobHash.Hex(), err)
		}
	}
	return nil
}

func (r *Wrapper) getEthCallOpts(ctx context.Context) (*bind.CallOpts, context.CancelFunc) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	return &bind.CallOpts{Context: ctxWithTimeout}, cancel
}

type contractCall func(opts *bind.CallOpts) error

// callWithTimeout executes a contract call with the specified timeout
func (r *Wrapper) callWithTimeout(ctx context.Context, call contractCall) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	defer cancel()

	return call(&bind.CallOpts{Context: ctxWithTimeout})
}

func (r *Wrapper) getEthTransactOpts(ctx context.Context) (*bind.TransactOpts, context.CancelFunc, error) {
	keyedTransactor, err := bind.NewKeyedTransactorWithChainID(r.privateKey, r.chainID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating keyed transactor with chain ID: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	keyedTransactor.Context = ctxWithTimeout
	return keyedTransactor, cancel, nil
}

// BatchValidation contains validation results for a batch
type BatchValidation struct {
	IsFinalized             bool
	IsCommitted             bool
	LastFinalizedBatchIndex string
	LastFinalizedStateRoot  common.Hash
}

func (r *Wrapper) validateBatch(
	ctx context.Context,
	batchIndex string,
	oldStateRoot common.Hash,
) (*BatchValidation, error) {
	validation := &BatchValidation{}

	// Check if batch is finalized
	if err := r.callWithTimeout(ctx, func(opts *bind.CallOpts) error {
		isFinalized, err := r.rollupContract.IsBatchFinalized(opts, batchIndex)
		validation.IsFinalized = isFinalized
		return err
	}); err != nil {
		return nil, err
	}

	if validation.IsFinalized {
		return nil, ErrBatchAlreadyFinalized
	}

	// Check if batch is committed
	if err := r.callWithTimeout(ctx, func(opts *bind.CallOpts) error {
		isCommitted, err := r.rollupContract.IsBatchCommitted(opts, batchIndex)
		validation.IsCommitted = isCommitted
		return err
	}); err != nil {
		return nil, err
	}

	if !validation.IsCommitted {
		return nil, fmt.Errorf("can't call UpdateState with uncommitted batch %s", batchIndex)
	}

	// Get last finalized batch index
	if err := r.callWithTimeout(ctx, func(opts *bind.CallOpts) error {
		index, err := r.rollupContract.LastFinalizedBatchIndex(opts)
		validation.LastFinalizedBatchIndex = index
		return err
	}); err != nil {
		return nil, err
	}

	// Get last finalized state root
	if err := r.callWithTimeout(ctx, func(opts *bind.CallOpts) error {
		stateRoot, err := r.rollupContract.FinalizedStateRoots(opts, validation.LastFinalizedBatchIndex)
		validation.LastFinalizedStateRoot = stateRoot
		return err
	}); err != nil {
		return nil, err
	}

	if !bytes.Equal(validation.LastFinalizedStateRoot[:], oldStateRoot.Bytes()) {
		return nil, fmt.Errorf("last finalized state root (%s) and oldStateRoot (%s) differ",
			validation.LastFinalizedStateRoot, oldStateRoot)
	}

	return validation, nil
}
