package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

const (
	MaxPriorityFeePerGas = int64(51331911220)
	MaxFeePerGas         = int64(153995733660)
	DefaultGas           = uint64(500000)
)

const abiJson = `
[
	{
		"type" : "function",
		"name" : "proofBatch",
		"inputs" : [
			{"name":"_prevStateRoot","type":"bytes32"},
			{"name":"_newStateRoot","type":"bytes32"},
			{"name":"_blobProof","type":"bytes"},
			{"name":"_batchIndexInBlobStorage","type":"uint256"}
		]
	}
]`

const abiFinalizedBatchIndexJson = `
[
	{
		"inputs" : [],
		"name" : "finalizedBatchIndex",
		"outputs" : [
			{"internalType" : "uint256","name" : "","type" : "uint256"}
		],
		"stateMutability" : "view",
		"type":"function"
	}
]`

const abiLatestProvedStateRootJson = `
[
	{
		"inputs" : [
			{"internalType" : "uint256","name" : "","type" : "uint256"}
		],
		"name" : "stateRoots",
		"outputs" : [
			{"internalType" : "bytes32","name" : "","type" : "bytes32"}
		],
		"stateMutability" : "view",
		"type" : "function"
	}
]`

type Proposer struct {
	client       client.RawClient
	blockStorage storage.BlockStorage
	retryRunner  common.RetryRunner

	seqno   atomic.Uint64
	chainId *big.Int
	params  *ProposerParams

	metrics ProposerMetrics
	logger  zerolog.Logger
}

const DefaultProposingInterval = 10 * time.Second

type ProposerParams struct {
	Endpoint          string
	ChainId           string
	PrivateKey        string
	ContractAddress   string
	SelfAddress       string
	ProposingInterval time.Duration
}

type ProposerMetrics interface {
	metrics.BasicMetrics
	RecordProposerTxSent(ctx context.Context, proposalData *scTypes.ProposalData)
}

func NewDefaultProposerParams() *ProposerParams {
	return &ProposerParams{
		Endpoint:          "http://rpc2.sepolia.org",
		ChainId:           "11155111",
		PrivateKey:        "0000000000000000000000000000000000000000000000000000000000000001",
		ContractAddress:   "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4",
		SelfAddress:       "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206",
		ProposingInterval: DefaultProposingInterval,
	}
}

func NewProposer(
	ctx context.Context,
	params *ProposerParams,
	rpcClient client.RawClient,
	blockStorage storage.BlockStorage,
	metrics ProposerMetrics,
	logger zerolog.Logger,
) (*Proposer, error) {
	nonceValue, err := getCurrentNonce(ctx, params.SelfAddress, rpcClient)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		logger.Error().Err(err).Msg("failed get current contract nonce, set 0")
	}

	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.ExponentialDelay(100*time.Millisecond, time.Second),
		},
		logger,
	)

	p := Proposer{
		client:       rpcClient,
		blockStorage: blockStorage,
		params:       params,
		retryRunner:  retryRunner,
		metrics:      metrics,
		logger:       logger,
	}
	var ok bool
	p.chainId, ok = new(big.Int).SetString(params.ChainId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid L1ChainId: %s", params.ChainId)
	}
	p.seqno.Add(nonceValue)

	p.logger.Info().Msgf(
		"ChainId %v\nL1Contract address %v\nNonce %d",
		p.chainId, p.params.ContractAddress, p.seqno.Load(),
	)
	return &p, nil
}

func (p *Proposer) Run(ctx context.Context) error {
	shouldResetStorage, err := p.initializeProvedStateRoot(ctx)
	if err != nil {
		return err
	}

	if shouldResetStorage {
		p.logger.Warn().Msg("resetting TaskStorage and BlockStorage")
		// todo: reset TaskStorage and BlockStorage before starting Aggregator, TaskScheduler and TaskListener
	}

	concurrent.RunTickerLoop(ctx, p.params.ProposingInterval,
		func(ctx context.Context) {
			if err := p.proposeNextBlock(ctx); err != nil {
				p.logger.Error().Err(err).Msg("error during proved blocks proposing")
				p.metrics.RecordError(ctx, "proposer")
				return
			}
		},
	)

	return nil
}

func (p *Proposer) initializeProvedStateRoot(ctx context.Context) (shouldResetStorage bool, err error) {
	storedStateRoot, err := p.blockStorage.TryGetProvedStateRoot(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check if proved state root is initialized: %w", err)
	}

	latestStateRoot, err := p.getLatestProvedStateRoot(ctx)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		p.logger.Error().Err(err).Msg("failed get current contract state root, set 0")
	}

	switch {
	case storedStateRoot == nil:
		p.logger.Info().
			Stringer("latestStateRoot", latestStateRoot).
			Msg("proved state root is not initialized, value from L1 will be used")
	case *storedStateRoot != latestStateRoot:
		p.logger.Warn().
			Uint64("seqno", p.seqno.Load()).
			Stringer("storedStateRoot", storedStateRoot).
			Stringer("latestStateRoot", latestStateRoot).
			Msg("proved state root value is invalid, local storage will be reset")
		shouldResetStorage = true
	default:
		p.logger.Info().Stringer("stateRoot", storedStateRoot).Msg("proved state root value is valid")
	}

	if storedStateRoot == nil || *storedStateRoot != latestStateRoot {
		err = p.blockStorage.SetProvedStateRoot(ctx, latestStateRoot)
		if err != nil {
			return false, fmt.Errorf("failed set proved state root: %w", err)
		}
	}

	p.logger.Info().
		Uint64("seqno", p.seqno.Load()).
		Stringer("stateRoot", latestStateRoot).
		Msg("proposer is initialized")
	return shouldResetStorage, nil
}

func (p *Proposer) proposeNextBlock(ctx context.Context) error {
	data, err := p.blockStorage.TryGetNextProposalData(ctx)
	if err != nil {
		return fmt.Errorf("failed get next block to propose: %w", err)
	}
	if data == nil {
		p.logger.Debug().Msg("no block to propose")
		return nil
	}

	err = p.sendProof(ctx, data)
	if err != nil {
		return fmt.Errorf("failed to send proof to L1 for block with hash=%s: %w", data.MainShardBlockHash, err)
	}

	blockId := scTypes.NewBlockId(types.MainShardId, data.MainShardBlockHash)
	err = p.blockStorage.SetBlockAsProposed(ctx, blockId)
	if err != nil {
		return fmt.Errorf("failed set block with hash=%s as proposed: %w", data.MainShardBlockHash, err)
	}
	return nil
}

func (p *Proposer) getLatestProvedStateRoot(ctx context.Context) (common.Hash, error) {
	// get finalizedBatchIndex
	finalizedBatchIndexAbi, err := abi.JSON(strings.NewReader(abiFinalizedBatchIndexJson))
	if err != nil {
		return common.EmptyHash, err
	}

	finalizedBatchIndexData, err := finalizedBatchIndexAbi.Pack("finalizedBatchIndex")
	if err != nil {
		return common.EmptyHash, err
	}

	valueStr := hexutil.EncodeBig(big.NewInt(0))
	msg := make(map[string]string)
	msg["from"] = p.params.SelfAddress
	msg["to"] = p.params.ContractAddress
	msg["gas"] = "0xc350"
	msg["gasPrice"] = "0x9184e72a000"
	msg["value"] = valueStr
	msg["data"] = hexutil.Encode(finalizedBatchIndexData)

	var response json.RawMessage
	err = p.retryRunner.Do(ctx, func(ctx context.Context) error {
		response, err = p.client.RawCall(ctx, "eth_call", msg, "latest")
		return err
	})
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed send eth_call for get finalizedBatchIndex: %w", err)
	}
	var finalizedBatchIndexStr string
	if err = json.Unmarshal(response, &finalizedBatchIndexStr); err != nil {
		return common.EmptyHash, fmt.Errorf("failed unmarshal finalizedBatchIndex: %w", err)
	}

	finalizedBatchIndexBytes, err := hexutil.Decode(finalizedBatchIndexStr)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed decode finalizedBatchIndex: %w", err)
	}

	finalizedBatchIndexValue := new(uint256.Int).SetBytes(finalizedBatchIndexBytes)
	finalizedBatchIndexValue, overflow := uint256.NewInt(0).SubOverflow(finalizedBatchIndexValue, uint256.NewInt(1))
	if overflow {
		return common.EmptyHash, errors.New("failed SubOverflow, overflow is true")
	}

	// get latestProvedStateRoot
	latestProvedStateRootAbi, err := abi.JSON(strings.NewReader(abiLatestProvedStateRootJson))
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed read JSON ABI for get latestProvedStateRoot: %w", err)
	}

	latestProvedStateRootData, err := latestProvedStateRootAbi.Pack("stateRoots", finalizedBatchIndexValue.ToBig())
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed pack JSON ABI for get latestProvedStateRoot: %w", err)
	}

	msg["data"] = hexutil.Encode(latestProvedStateRootData)

	var latestProvedStateRoot json.RawMessage
	err = p.retryRunner.Do(ctx, func(ctx context.Context) error {
		latestProvedStateRoot, err = p.client.RawCall(ctx, "eth_call", msg, "latest")
		return err
	})
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed send eth_call for getting the latest proved state root: %w", err)
	}

	var latestProvedStateRootStr string
	err = json.Unmarshal(latestProvedStateRoot, &latestProvedStateRootStr)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed decode the latest proved state root: %w", err)
	}

	return common.HexToHash(latestProvedStateRootStr), nil
}

func getCurrentNonce(ctx context.Context, selfAddress string, client client.RawClient) (uint64, error) {
	res, err := client.RawCall(ctx, "eth_getTransactionCount", selfAddress, "latest")
	if err != nil {
		return 0, fmt.Errorf("failed send eth_getTransactionCount: %w", err)
	}
	if res == nil {
		return 0, nil
	}

	var nonceValue hexutil.Uint64
	if err = nonceValue.UnmarshalJSON(res); err != nil {
		return 0, fmt.Errorf("failed unmarshal result: %w", err)
	}
	return (uint64)(nonceValue), nil
}

func (p *Proposer) createUpdateStateTransaction(provedStateRoot, newStateRoot common.Hash) (*ethTypes.Transaction, error) {
	if provedStateRoot.Empty() || newStateRoot.Empty() {
		return nil, fmt.Errorf("empty hash for state update transaction %d", p.seqno.Load())
	}

	abiInterface, err := abi.JSON(strings.NewReader(abiJson))
	if err != nil {
		return nil, err
	}

	proof := make([]byte, 0)
	data, err := abiInterface.Pack("proofBatch", provedStateRoot, newStateRoot, proof, big.NewInt(0))
	if err != nil {
		return nil, err
	}

	L1ContractAddress := ethcommon.HexToAddress(p.params.ContractAddress)
	transaction := ethTypes.NewTx(&ethTypes.DynamicFeeTx{
		ChainID:   p.chainId,
		Nonce:     p.seqno.Load(),
		GasTipCap: big.NewInt(MaxPriorityFeePerGas),
		GasFeeCap: big.NewInt(MaxFeePerGas),
		Gas:       DefaultGas,
		To:        &L1ContractAddress,
		Value:     big.NewInt(0),
		Data:      data,
	})

	privateKey, err := crypto.HexToECDSA(p.params.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key %w", err)
	}

	signedTx, err := ethTypes.SignTx(transaction, ethTypes.NewCancunSigner(p.chainId), privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction %w", err)
	}

	return signedTx, nil
}

func (p *Proposer) encodeTransaction(transaction *ethTypes.Transaction) (string, error) {
	encodedTransaction, err := transaction.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to encode StateUpdateTransaction: %w", err)
	}

	return hexutil.Encode(encodedTransaction), nil
}

func (p *Proposer) sendProof(ctx context.Context, data *scTypes.ProposalData) error {
	p.logger.Info().
		Stringer("blockHash", data.MainShardBlockHash).
		Int64("seqno", int64(p.seqno.Load())).
		Int("txCount", len(data.Transactions)).
		Msgf("sending proof to L1")

	signedTx, err := p.createUpdateStateTransaction(data.OldProvedStateRoot, data.NewProvedStateRoot)
	if err != nil {
		return fmt.Errorf("failed create StateUpdateTransaction %w", err)
	}

	encodedTransactionStr, err := p.encodeTransaction(signedTx)
	if err != nil {
		return fmt.Errorf("failed encode StateUpdateTransaction %w", err)
	}

	p.logger.Debug().Msg(encodedTransactionStr)

	// call UpdateState L1 contract
	err = p.retryRunner.Do(ctx, func(context.Context) error {
		_, err := p.client.RawCall(ctx, "eth_sendRawTransaction", encodedTransactionStr)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update state (eth_sendRawTransaction): %w", err)
	}
	p.seqno.Add(1)
	// TODO send bloob with transactions and KZG proof

	p.metrics.RecordProposerTxSent(ctx, data)
	return nil
}
