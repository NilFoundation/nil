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

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

type Proposer interface {
	SendProof(ctx context.Context, provedStateRoot, newStateRoot common.Hash, transactions []*storage.PrunedTransaction) error
}

type proposerImpl struct {
	l1EndPoint            string
	client                *rpc.Client
	seqno                 atomic.Uint64
	chainId               *big.Int
	privKey               string
	contractAddress       string
	latestProvedStateRoot common.Hash

	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

type ProposerParams struct {
	endpoint        string
	chainId         string
	privKey         string
	contractAddress string
	selfAddress     string
}

func DefaultProposerParams() ProposerParams {
	return ProposerParams{
		endpoint:        "http://rpc2.sepolia.org",
		chainId:         "11155111",
		privKey:         "0000000000000000000000000000000000000000000000000000000000000001",
		contractAddress: "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4",
		selfAddress:     "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206",
	}
}

func newProposer(params ProposerParams, logger zerolog.Logger) (*proposerImpl, error) {
	client := rpc.NewClient(params.endpoint, logger)

	chainId, ok := new(big.Int).SetString(params.chainId, 10)
	if !ok {
		return nil, fmt.Errorf("wrong chainId: %s", params.chainId)
	}

	nonceValue, err := getCurrentNonce(params.selfAddress, client)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		logger.Error().Err(err).Msg("failed get current contract nonce, set 0")
	}

	latestProvedStateRoot, err := getLatestProvedStateRoot(params.selfAddress, params.contractAddress, client)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		logger.Error().Err(err).Msg("failed get current contract state root, set 0")
	}

	logger.Debug().Int64("seqno", int64(nonceValue)).Msg("initialize proposer, latestProvedStateRoot = " + latestProvedStateRoot.String())

	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.ExponentialDelay(100*time.Millisecond, time.Second),
		},
		logger,
	)

	p := proposerImpl{
		client:                client,
		l1EndPoint:            params.endpoint,
		chainId:               chainId,
		privKey:               params.privKey,
		contractAddress:       params.contractAddress,
		latestProvedStateRoot: latestProvedStateRoot,
		retryRunner:           retryRunner,
		logger:                logger,
	}
	p.seqno.Add(nonceValue)
	return &p, nil
}

func getLatestProvedStateRoot(selfAddress string, contractAddress string, client *rpc.Client) (common.Hash, error) {
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
	msg["from"] = selfAddress
	msg["to"] = contractAddress
	msg["gas"] = "0xc350"
	msg["gasPrice"] = "0x9184e72a000"
	msg["value"] = valueStr
	msg["data"] = hexutil.Encode(finalizedBatchIndexData)

	res, err := client.RawCall("eth_call", msg, "latest")
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed send eth_call for get finalizedBatchIndex: %w", err)
	}
	var finalizedBatchIndexStr string
	if err = json.Unmarshal(res, &finalizedBatchIndexStr); err != nil {
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
	latestProvedStateRoot, err := client.RawCall("eth_call", msg, "latest")
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed send eth_call for getting the latest proved state root: %w", err)
	}

	return common.BytesToHash(latestProvedStateRoot), nil
}

func getCurrentNonce(selfAddress string, client *rpc.Client) (uint64, error) {
	res, err := client.RawCall("eth_getTransactionCount", selfAddress, "latest")
	if err != nil {
		return 0, fmt.Errorf("failed send eth_getTransactionCount: %w", err)
	}
	var nonceValue hexutil.Uint64
	if err = nonceValue.UnmarshalJSON(res); err != nil {
		return 0, fmt.Errorf("failed unmarshal result: %w", err)
	}
	return (uint64)(nonceValue), nil
}

func (p *proposerImpl) createUpdateStateTransaction(provedStateRoot, newStateRoot common.Hash) (*types.Transaction, error) {
	if provedStateRoot.Empty() || newStateRoot.Empty() {
		return nil, fmt.Errorf("empty hash for state update transaction %d", p.seqno.Load())
	}

	abi, err := abi.JSON(strings.NewReader(abiJson))
	if err != nil {
		return nil, err
	}

	proof := make([]byte, 0)
	data, err := abi.Pack("proofBatch", provedStateRoot, newStateRoot, proof, big.NewInt(0))
	if err != nil {
		return nil, err
	}

	L1ContractAddress := ethcommon.HexToAddress(p.contractAddress)
	transaction := types.NewTx(&types.DynamicFeeTx{
		ChainID:   p.chainId,
		Nonce:     p.seqno.Load(),
		GasTipCap: big.NewInt(MaxPriorityFeePerGas),
		GasFeeCap: big.NewInt(MaxFeePerGas),
		Gas:       DefaultGas,
		To:        &L1ContractAddress,
		Value:     big.NewInt(0),
		Data:      data,
	})

	privateKey, err := crypto.HexToECDSA(p.privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key %w", err)
	}

	signedTx, err := types.SignTx(transaction, types.NewCancunSigner(p.chainId), privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction %w", err)
	}

	return signedTx, nil
}

func (p *proposerImpl) encodeTransaction(transaction *types.Transaction) (string, error) {
	encodedTransaction, err := transaction.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to encode StateUpdateTransaction: %w", err)
	}

	return hexutil.Encode(encodedTransaction), nil
}

func (p *proposerImpl) SendProof(ctx context.Context, provedStateRoot, newStateRoot common.Hash, transactions []*storage.PrunedTransaction) error {
	p.logger.Debug().Int64("seqno", int64(p.seqno.Load())).Int64("transactionsCount", int64(len(transactions))).Msg("skip processing transactions")

	signedTx, err := p.createUpdateStateTransaction(provedStateRoot, newStateRoot)
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
		_, err := p.client.RawCall("eth_sendRawTransaction", encodedTransactionStr)
		return err
	})
	if err != nil {
		// TODO return error after enable local L1 endpoint
		p.logger.Error().Err(err).Msg("failed update state on L1 request")
	}
	p.seqno.Add(1)
	// TODO send bloob with transactions and KZG proof
	return nil
}
