package rollupcontract

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/l1client"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Wrapper interface {
	SetGenesisStateRoot(ctx context.Context, genesisStateRoot common.Hash) error

	UpdateState(ctx context.Context, data *types.UpdateStateData) error

	GetLatestFinalizedStateRoot(ctx context.Context) (common.Hash, error)

	VerifyDataProofs(ctx context.Context, commitment *batches.Commitment) error

	CommitBatch(ctx context.Context, batchId types.BatchId, sidecar *ethtypes.BlobTxSidecar) error

	RollbackState(ctx context.Context, targetRoot common.Hash) error
}

const (
	DefaultEndpoint        = "http://rpc2.sepolia.org"
	DefaultPrivateKey      = "0000000000000000000000000000000000000000000000000000000000000001"
	DefaultContractAddress = "0xBa79C93859394a5DEd3c1132a87f706Cca2582aA"
	DefaultRequestTimeout  = 10 * time.Second
)

type WrapperConfig struct {
	Endpoint           string        `yaml:"l1Endpoint,omitempty"`
	RequestsTimeout    time.Duration `yaml:"l1ClientTimeout,omitempty"`
	DisableL1          bool          `yaml:"disableL1,omitempty"`
	PrivateKeyHex      string        `yaml:"l1PrivateKey,omitempty"`
	ContractAddressHex string        `yaml:"l1ContractAddress,omitempty"`
}

func NewWrapperConfig(
	endpoint string,
	privateKeyHex string,
	contractAddressHex string,
	requestsTimeout time.Duration,
	disableL1 bool,
) WrapperConfig {
	return WrapperConfig{
		Endpoint:           endpoint,
		PrivateKeyHex:      privateKeyHex,
		ContractAddressHex: contractAddressHex,
		RequestsTimeout:    requestsTimeout,
		DisableL1:          disableL1,
	}
}

func NewDefaultWrapperConfig() WrapperConfig {
	return NewWrapperConfig(
		DefaultEndpoint,
		DefaultPrivateKey,
		DefaultContractAddress,
		DefaultRequestTimeout,
		false,
	)
}

type wrapperImpl struct {
	rollupContract  *Rollupcontract
	contractAddress ethcommon.Address
	senderAddress   ethcommon.Address
	privateKey      *ecdsa.PrivateKey
	chainID         *big.Int
	ethClient       l1client.EthClient
	abi             *abi.ABI
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
	var ethClient l1client.EthClient
	if cfg.DisableL1 {
		return &noopWrapper{
			logger: logger,
		}, nil
	}

	ethClient, err := l1client.NewRetryingEthClient(ctx, cfg.Endpoint, cfg.RequestsTimeout, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing eth client: %w", err)
	}

	return NewWrapperWithEthClient(ctx, cfg, ethClient, logger)
}

func NewWrapperWithEthClient(
	ctx context.Context,
	cfg WrapperConfig,
	ethClient l1client.EthClient,
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

	abiDefinition, err := RollupcontractMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("getting ABI: %w", err)
	}

	publicKeyECDSA, ok := privateKeyECDSA.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}
	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &wrapperImpl{
		rollupContract:  rollupContract,
		contractAddress: contactAddress,
		senderAddress:   senderAddress,
		privateKey:      privateKeyECDSA,
		chainID:         chainID,
		ethClient:       ethClient,
		abi:             abiDefinition,
		logger:          logger,
	}, nil
}

func (r *wrapperImpl) SetGenesisStateRoot(ctx context.Context, genesisStateRoot common.Hash) error {
	latestFinalizedStateRoot, err := r.GetLatestFinalizedStateRoot(ctx)
	if err != nil {
		return err
	}
	if latestFinalizedStateRoot != common.EmptyHash {
		return fmt.Errorf("%w: %s", types.ErrL1StateRootAlreadyInitialized, latestFinalizedStateRoot)
	}

	var tx *ethtypes.Transaction
	if err := r.transactWithCtx(ctx, func(opts *bind.TransactOpts) error {
		var err error
		tx, err = r.rollupContract.SetGenesisStateRoot(
			opts,
			genesisStateRoot,
		)
		return err
	}); err != nil {
		return fmt.Errorf("SetGenesisStateRoot transaction failed: %w", err)
	}
	r.logger.Info().
		Hex("txHash", tx.Hash().Bytes()).
		Int("gasLimit", int(tx.Gas())).
		Int("cost", int(tx.Cost().Uint64())).
		Msg("SetGenesisStateRoot transaction sent")

	receipt, err := r.waitForReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("error during waiting for receipt: %w", err)
	}
	r.logReceiptDetails(receipt)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return errors.New("SetGenesisStateRoot tx failed")
	}

	return err
}

func (r *wrapperImpl) GetLatestFinalizedStateRoot(ctx context.Context) (common.Hash, error) {
	latestFinalizedBatchIndex, err := r.latestFinalizedBatchIndex(ctx)
	if err != nil {
		return common.EmptyHash, err
	}

	return r.rollupContract.FinalizedStateRoots(r.getEthCallOpts(ctx), latestFinalizedBatchIndex)
}

func (r *wrapperImpl) latestFinalizedBatchIndex(ctx context.Context) (string, error) {
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

	// Any execution-level error (e.g., reverts, failed require statements) would have been triggered
	// during eth_estimateGas, which is implicitly called before sending the transaction.
	// Such errors can be parsed similarly to those from eth_call, and in those cases,
	// the transaction is never broadcast to the network.
	if err := transactFunc(transactOpts); err != nil {
		return r.decodeContractError(err)
	}
	return nil
}

func (r *wrapperImpl) signTx(tx *ethtypes.Transaction) (*ethtypes.Transaction, error) {
	keyedTransactor, err := r.getKeyedTransactor()
	if err != nil {
		return nil, err
	}

	return keyedTransactor.Signer(r.senderAddress, tx)
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

// RollbackState resets contract state to specified `targetRoot`.
func (r *wrapperImpl) RollbackState(ctx context.Context, targetRoot common.Hash) error {
	var tx *ethtypes.Transaction
	if err := r.transactWithCtx(ctx, func(opts *bind.TransactOpts) error {
		var err error
		tx, err = r.rollupContract.ResetState(
			opts,
			targetRoot,
		)
		return err
	}); err != nil {
		return fmt.Errorf("RollbackState transaction failed: %w", err)
	}
	r.logger.Info().
		Hex("txHash", tx.Hash().Bytes()).
		Int("gasLimit", int(tx.Gas())).
		Int("cost", int(tx.Cost().Uint64())).
		Msg("RollbackState transaction sent")

	receipt, err := r.waitForReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("error during waiting for receipt: %w", err)
	}
	r.logReceiptDetails(receipt)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return errors.New("RollbackState tx failed")
	}

	return err
}

type contractError struct {
	MethodName string
	Args       map[string]any
}

func (e contractError) Error() string {
	return fmt.Sprintf("contract error: %s with args %v", e.MethodName, e.Args)
}

func (r *wrapperImpl) decodeContractError(err error) error {
	revertData, errorDecoded := ethclient.RevertErrorData(err)
	if !errorDecoded {
		return fmt.Errorf("error couldn't be decoded: %w", err)
	}

	if len(revertData) < 4 {
		return fmt.Errorf("not enough data to unparse error: %w", err)
	}
	var selector [4]byte
	copy(selector[:], revertData[:4])

	errorMethod, err := r.abi.ErrorByID(selector)
	if err != nil {
		return err
	}

	args := make(map[string]any)
	err = errorMethod.Inputs.UnpackIntoMap(args, revertData[4:])
	if err != nil {
		return fmt.Errorf("args upack error: %w", err)
	}

	return contractError{errorMethod.Name, args}
}

// abigen doesn't generate error types, have to specify them manually
var contractErrorMap = map[string]error{
	"ErrorGenesisStateRootIsNotInitialized":          types.ErrL1StateRootNotInitialized,
	"ErrorInvalidBatchIndex":                         ErrInvalidBatchIndex,
	"ErrorInvalidOldStateRoot":                       ErrInvalidOldStateRoot,
	"ErrorInvalidNewStateRoot":                       ErrInvalidNewStateRoot,
	"ErrorInvalidValidityProof":                      ErrInvalidValidityProof,
	"ErrorEmptyDataProofs":                           ErrEmptyDataProofs,
	"ErrorBatchNotCommitted":                         ErrBatchNotCommitted,
	"ErrorBatchAlreadyFinalized":                     ErrBatchAlreadyFinalized,
	"ErrorBatchAlreadyCommitted":                     ErrBatchAlreadyCommitted,
	"ErrorDataProofsAndBlobCountMismatch":            ErrDataProofsAndBlobCountMismatch,
	"ErrorNewStateRootAlreadyFinalized":              ErrNewStateRootAlreadyFinalized,
	"ErrorOldStateRootMismatch":                      ErrOldStateRootMismatch,
	"ErrorIncorrectDataProofSize":                    ErrIncorrectDataProofSize,
	"ErrorL1BridgeMessengerAddressNotSet":            ErrL1BridgeMessengerAddressNotSet,
	"ErrorInconsistentDepositNonce":                  ErrInconsistentDepositNonce,
	"ErrorInvalidPublicDataInfo":                     ErrInvalidPublicDataInfo,
	"ErrorL1MessageHashMismatch":                     ErrL1MessageHashMismatch,
	"ErrorInvalidDataProofItem":                      ErrInvalidDataProofItem,
	"ErrorInvalidPublicInputForProof":                ErrInvalidPublicInputForProof,
	"ErrorCallPointEvaluationPrecompileFailed":       ErrCallPointEvaluationPrecompileFailed,
	"ErrorUnexpectedPointEvaluationPrecompileOutput": ErrUnexpectedPointEvaluationPrecompileOutput,
	"ErrorInvalidVersionedHash":                      ErrInvalidVersionedHash,
}

// errorByName looks for specific error type by its name, returns it if found, otherwise, returns
// the initial error.
func (r *wrapperImpl) errorByName(err error) error {
	var cerr contractError
	if errors.As(err, &cerr) {
		if mappedErr, ok := contractErrorMap[cerr.MethodName]; ok {
			return mappedErr
		}
	}
	return err
}

// simulateTx simulates transaction using `eth_call` method, tries to decode error
func (r *wrapperImpl) simulateTx(ctx context.Context, tx *ethtypes.Transaction, blockNumber *big.Int) error {
	args := map[string]any{
		"from":     r.senderAddress,
		"to":       tx.To(),
		"gas":      hexutil.Uint64(tx.Gas()),
		"gasPrice": hexutil.Big(*tx.GasPrice()),
		"value":    hexutil.Big(*tx.Value()),
		"data":     hexutil.Bytes(tx.Data()),
	}

	if sidecar := tx.BlobTxSidecar(); sidecar != nil {
		args["blobs"] = sidecar.Blobs
		args["kzgCommitments"] = sidecar.Commitments
		args["kzgProofs"] = sidecar.Proofs
		args["blobVersionedHashes"] = tx.BlobHashes()
		args["blobFeeCap"] = (*hexutil.Big)(tx.BlobGasFeeCap())
	}

	var result any
	err := r.ethClient.RawCall(ctx, result, "eth_call", args, toBlockNumArg(blockNumber))
	if err != nil {
		return r.decodeContractError(err)
	}

	return nil
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	return hexutil.EncodeBig(number)
}

type noopWrapper struct {
	logger    logging.Logger
	stateRoot common.Hash
	mutex     sync.RWMutex
}

var _ Wrapper = (*noopWrapper)(nil)

func (w *noopWrapper) SetGenesisStateRoot(_ context.Context, hash common.Hash) error {
	w.logger.Debug().Msg("SetGenesisStateRoot noop wrapper method called")

	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.stateRoot != common.EmptyHash {
		return fmt.Errorf("%w: %s", types.ErrL1StateRootAlreadyInitialized, w.stateRoot)
	}
	w.stateRoot = hash

	return nil
}

func (w *noopWrapper) UpdateState(_ context.Context, data *types.UpdateStateData) error {
	w.logger.Debug().Msg("UpdateState noop wrapper method called")

	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.stateRoot == common.EmptyHash {
		return types.ErrL1StateRootNotInitialized
	}

	if w.stateRoot != data.OldProvedStateRoot {
		return fmt.Errorf(
			"%w, currentState=%s, received=%s", ErrOldStateRootMismatch, w.stateRoot, data.OldProvedStateRoot,
		)
	}

	w.stateRoot = data.NewProvedStateRoot

	return nil
}

func (w *noopWrapper) GetLatestFinalizedStateRoot(context.Context) (common.Hash, error) {
	w.logger.Debug().Msg("FinalizedStateRoot noop wrapper method called")

	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.stateRoot, nil
}

func (w *noopWrapper) VerifyDataProofs(context.Context, *batches.Commitment) error {
	w.logger.Debug().Msg("VerifyDataProofs noop wrapper method called")
	return nil
}

func (w *noopWrapper) CommitBatch(context.Context, types.BatchId, *ethtypes.BlobTxSidecar) error {
	w.logger.Debug().Msg("CommitBatch noop wrapper method called")
	return nil
}

func (w *noopWrapper) RollbackState(context.Context, common.Hash) error {
	w.logger.Debug().Msg("RollbackState noop wrapper method called")
	return nil
}
