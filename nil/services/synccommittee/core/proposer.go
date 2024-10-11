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
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
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
	client       *rpc.Client
	blockStorage storage.BlockStorage
	seqno        atomic.Uint64
	chainId      *big.Int
	params       ProposerParams
	retryRunner  common.RetryRunner
	logger       zerolog.Logger
}

const DefaultProposingInterval = 10 * time.Second

type ProposerParams struct {
	endpoint          string
	chainId           string
	privKey           string
	contractAddress   string
	selfAddress       string
	proposingInterval time.Duration
}

func DefaultProposerParams() ProposerParams {
	return ProposerParams{
		endpoint:          "http://rpc2.sepolia.org",
		chainId:           "11155111",
		privKey:           "0000000000000000000000000000000000000000000000000000000000000001",
		contractAddress:   "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4",
		selfAddress:       "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206",
		proposingInterval: DefaultProposingInterval,
	}
}

func NewProposer(params ProposerParams, blockStorage storage.BlockStorage, logger zerolog.Logger) (*Proposer, error) {
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

	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.ExponentialDelay(100*time.Millisecond, time.Second),
		},
		logger,
	)

	p := Proposer{
		client:       client,
		blockStorage: blockStorage,
		chainId:      chainId,
		params:       params,
		retryRunner:  retryRunner,
		logger:       logger,
	}
	p.seqno.Add(nonceValue)

	p.logger.Info().Msgf("\nUse L1 endpoint %v\nChainId %v\nL1Contract address %v\nLatestProvedStateRoot %s\nNonce %d",
		p.l1EndPoint, params.chainId, p.contractAddress, latestProvedStateRoot, p.seqno.Load())
	return &p, nil
}

func (p *Proposer) Run(ctx context.Context) error {
	isInitialized, err := p.blockStorage.ProvedStateRootIsInitialized(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if proved state root is initialized: %w", err)
	}

	if !*isInitialized {
		latestProvedStateRoot, err := p.getLatestProvedStateRoot(ctx)
		if err != nil {
			// TODO return error after enable local L1 endpoint
			p.logger.Error().Err(err).Msg("failed get current contract state root, set 0")
		}

		p.logger.Debug().
			Uint64("seqno", p.seqno.Load()).
			Str("latestProvedStateRoot", latestProvedStateRoot.String()).
			Msg("proposer is initialized")
	}

	concurrent.RunTickerLoop(ctx, p.params.proposingInterval,
		func(ctx context.Context) {
			if err := p.proposeNextBlock(ctx); err != nil {
				p.logger.Error().Err(err).Msg("error during proved blocks proposing")
				return
			}
		},
	)

	return nil
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

	err = p.blockStorage.SetBlockAsProposed(ctx, data.MainShardBlockHash)
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
	msg["from"] = p.params.selfAddress
	msg["to"] = p.params.contractAddress
	msg["gas"] = "0xc350"
	msg["gasPrice"] = "0x9184e72a000"
	msg["value"] = valueStr
	msg["data"] = hexutil.Encode(finalizedBatchIndexData)

	var response json.RawMessage
	err = p.retryRunner.Do(ctx, func(ctx context.Context) error {
		response, err = p.client.RawCall("eth_call", msg, "latest")
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
		latestProvedStateRoot, err = p.client.RawCall("eth_call", msg, "latest")
		return err
	})
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

	L1ContractAddress := ethcommon.HexToAddress(p.params.contractAddress)
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

	privateKey, err := crypto.HexToECDSA(p.params.privKey)
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

func (p *Proposer) sendProof(ctx context.Context, data *storage.ProposalData) error {
	p.logger.Debug().
		Str("blockHash", data.MainShardBlockHash.String()).
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
