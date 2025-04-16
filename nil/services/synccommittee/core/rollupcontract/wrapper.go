package rollupcontract

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type Wrapper interface {
	UpdateState(
		ctx context.Context,
		batchIndex string,
		dataProofs types.DataProofs,
		oldStateRoot, newStateRoot common.Hash,
		validityProof []byte,
		publicDataInputs INilRollupPublicDataInfo,
	) error
	FinalizedStateRoot(ctx context.Context, finalizedBatchIndex string) (common.Hash, error)
	FinalizedBatchIndex(ctx context.Context) (string, error)
	CommitBatch(
		ctx context.Context,
		sidecar *ethtypes.BlobTxSidecar,
		batchIndex string,
	) error
	PrepareBlobs(ctx context.Context, blobs []kzg4844.Blob) (*ethtypes.BlobTxSidecar, types.DataProofs, error)
	ResetState(ctx context.Context, targetRoot common.Hash) error
}

type WrapperConfig struct {
	Endpoint           string        `yaml:"l1Endpoint,omitempty"`
	RequestsTimeout    time.Duration `yaml:"l1ClientTimeout,omitempty"`
	DisableL1          bool          `yaml:"disableL1,omitempty"`
	PrivateKeyHex      string        `yaml:"l1PrivateKey,omitempty"`
	ContractAddressHex string        `yaml:"l1ContractAddress,omitempty"`
}

func NewDefaultWrapperConfig() WrapperConfig {
	return WrapperConfig{
		Endpoint:           "http://rpc2.sepolia.org",
		RequestsTimeout:    10 * time.Second,
		DisableL1:          false,
		PrivateKeyHex:      "0000000000000000000000000000000000000000000000000000000000000001",
		ContractAddressHex: "0xBa79C93859394a5DEd3c1132a87f706Cca2582aA",
	}
}

type wrapperImpl struct {
	rollupContract  *Rollupcontract
	contractAddress ethcommon.Address
	privateKey      *ecdsa.PrivateKey
	chainID         *big.Int
	ethClient       EthClient
	logger          logging.Logger
}

var _ Wrapper = (*wrapperImpl)(nil)

// NewWrapper initializes a Wrapper for interacting with an Ethereum contract.
// It converts contract and private key hex strings to Ethereum formats, sets up the contract instance,
// and fetches the Ethereum client's chain ID.
func NewWrapper(
	ctx context.Context,
	cfg WrapperConfig,
	logger logging.Logger,
) (Wrapper, error) {
	var ethClient EthClient
	if cfg.DisableL1 {
		return &noopWrapper{logger}, nil
	}

	ethClient, err := NewRetryingEthClient(ctx, cfg.Endpoint, cfg.RequestsTimeout, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing eth client: %w", err)
	}

	return NewWrapperWithEthClient(ctx, cfg, ethClient, logger)
}

func NewWrapperWithEthClient(
	ctx context.Context,
	cfg WrapperConfig,
	ethClient EthClient,
	logger logging.Logger,
) (Wrapper, error) {
	contactAddress := ethcommon.HexToAddress(cfg.ContractAddressHex)
	rollupContract, err := NewRollupcontract(contactAddress, ethClient)
	if err != nil {
		return nil, fmt.Errorf("can't create rollup contract instance: %w", err)
	}

	privateKeyECDSA, err := crypto.HexToECDSA(cfg.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("converting private key hex to ECDSA: %w", err)
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chain ID: %w", err)
	}

	return &wrapperImpl{
		rollupContract:  rollupContract,
		contractAddress: contactAddress,
		privateKey:      privateKeyECDSA,
		chainID:         chainID,
		ethClient:       ethClient,
		logger:          logger,
	}, nil
}

func (r *wrapperImpl) FinalizedStateRoot(ctx context.Context, finalizedBatchIndex string) (common.Hash, error) {
	return r.rollupContract.FinalizedStateRoots(r.getEthCallOpts(ctx), finalizedBatchIndex)
}

func (r *wrapperImpl) FinalizedBatchIndex(ctx context.Context) (string, error) {
	return r.rollupContract.GetLastFinalizedBatchIndex(r.getEthCallOpts(ctx))
}

func (r *wrapperImpl) getEthCallOpts(ctx context.Context) *bind.CallOpts {
	return &bind.CallOpts{Context: ctx}
}

type (
	contractTransactFunc func(opts *bind.TransactOpts) error
)

func (r *wrapperImpl) getKeyedTransactor() (*bind.TransactOpts, error) {
	keyedTransactor, err := bind.NewKeyedTransactorWithChainID(r.privateKey, r.chainID)
	if err != nil {
		return nil, fmt.Errorf("creating keyed transactor with chain ID: %w", err)
	}

	return keyedTransactor, nil
}

func (r *wrapperImpl) getEthTransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	transactOpts, err := r.getKeyedTransactor()
	if err != nil {
		return nil, err
	}
	transactOpts.Context = ctx
	return transactOpts, nil
}

func (r *wrapperImpl) transactWithCtx(ctx context.Context, transactFunc contractTransactFunc) error {
	transactOpts, err := r.getEthTransactOpts(ctx)
	if err != nil {
		return err
	}

	return transactFunc(transactOpts)
}

// waitForReceipt repeatedly tries to get tx receipt, retrying on `NotFound` error (tx not mined yet).
// In case `ReceiptWaitFor` timeout is reached, raises an error.
func (r *wrapperImpl) waitForReceipt(ctx context.Context, txnHash ethcommon.Hash) (*ethtypes.Receipt, error) {
	const (
		ReceiptWaitFor  = 30 * time.Second
		ReceiptWaitTick = 500 * time.Millisecond
	)
	receipt, err := common.WaitForValue(
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
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, errors.New("waitForReceipt timeout reached")
	}
	return receipt, nil
}

// logReceiptDetails logs the essential details of a transaction receipt.
func (r *wrapperImpl) logReceiptDetails(receipt *ethtypes.Receipt) {
	r.logger.Info().
		Uint8("type", receipt.Type).
		Uint64("status", receipt.Status).
		Uint64("cumulativeGasUsed", receipt.CumulativeGasUsed).
		Hex("txHash", receipt.TxHash.Bytes()).
		Str("contractAddress", receipt.ContractAddress.Hex()).
		Uint64("gasUsed", receipt.GasUsed).
		Str("effectiveGasPrice", receipt.EffectiveGasPrice.String()).
		Hex("blockHash", receipt.BlockHash.Bytes()).
		Str("blockNumber", receipt.BlockNumber.String()).
		Uint("transactionIndex", receipt.TransactionIndex).
		Msg("transaction receipt received")
}

// ResetState resets contract state to specified `targetRoot`.
func (r *wrapperImpl) ResetState(ctx context.Context, targetRoot common.Hash) error {
	var tx *ethtypes.Transaction
	if err := r.transactWithCtx(ctx, func(opts *bind.TransactOpts) error {
		var err error
		tx, err = r.rollupContract.ResetState(
			opts,
			targetRoot,
		)
		return err
	}); err != nil {
		return fmt.Errorf("ResetState transaction failed: %w", err)
	}
	r.logger.Info().
		Hex("txHash", tx.Hash().Bytes()).
		Int("gasLimit", int(tx.Gas())).
		Int("cost", int(tx.Cost().Uint64())).
		Msg("ResetState transaction sent")

	receipt, err := r.waitForReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("error during waiting for receipt: %w", err)
	}
	r.logReceiptDetails(receipt)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return errors.New("ResetState tx failed")
	}

	return err
}

type noopWrapper struct {
	logger logging.Logger
}

var _ Wrapper = (*noopWrapper)(nil)

func (w *noopWrapper) UpdateState(
	ctx context.Context,
	batchIndex string,
	dataProofs types.DataProofs,
	oldStateRoot, newStateRoot common.Hash,
	validityProof []byte,
	publicDataInputs INilRollupPublicDataInfo,
) error {
	w.logger.Debug().Msg("UpdateState noop wrapper method called")
	return nil
}

func (w *noopWrapper) FinalizedStateRoot(ctx context.Context, finalizedBatchIndex string) (common.Hash, error) {
	w.logger.Debug().Msg("FinalizedStateRoot noop wrapper method called")
	return common.Hash{}, nil
}

func (w *noopWrapper) FinalizedBatchIndex(ctx context.Context) (string, error) {
	w.logger.Debug().Msg("FinalizedBatchIndex noop wrapper method called")
	return "", nil
}

func (w *noopWrapper) PrepareBlobs(
	ctx context.Context, blobs []kzg4844.Blob,
) (*ethtypes.BlobTxSidecar, types.DataProofs, error) {
	w.logger.Debug().Msg("PrepareBlobs noop wrapper method called")
	return nil, nil, nil
}

func (w *noopWrapper) CommitBatch(
	ctx context.Context,
	sidecar *ethtypes.BlobTxSidecar,
	batchIndex string,
) error {
	w.logger.Debug().Msg("CommitBatch noop wrapper method called")
	return nil
}

func (w *noopWrapper) ResetState(ctx context.Context, targetRoot common.Hash) error {
	w.logger.Debug().Msg("ResetState noop wrapper method called")
	return nil
}
