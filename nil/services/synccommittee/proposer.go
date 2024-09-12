package synccommittee

import (
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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

type Proposer struct {
	l1EndPoint      string
	client          *rpc.Client
	seqno           atomic.Uint64
	chainId         *big.Int
	privKey         string
	contractAddress string
	logger          zerolog.Logger
}

func NewProposer(endpoint string, chainIdStr string, privKey string, contractAddress string, logger zerolog.Logger) (*Proposer, error) {
	client := rpc.NewClient(endpoint, logger)

	chainId, ok := new(big.Int).SetString(chainIdStr, 10)
	if !ok {
		return nil, fmt.Errorf("wrong chainId: %s", chainIdStr)
	}
	return &Proposer{
		client:          client,
		l1EndPoint:      endpoint,
		chainId:         chainId,
		privKey:         privKey,
		contractAddress: contractAddress,
		logger:          logger,
	}, nil
}

func (p *Proposer) createUpdateStateTransaction(provedStateRoot, newStateRoot common.Hash) (*types.Transaction, error) {
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

func (p *Proposer) encodeTransaction(transaction *types.Transaction) (string, error) {
	encodedTransaction, err := transaction.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to encode StateUpdateTransaction: %w", err)
	}

	return hexutil.Encode(encodedTransaction), nil
}

func (p *Proposer) SendProof(provedStateRoot, newStateRoot common.Hash, transactions []*storage.PrunedTransaction) error {
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
	_, err = p.client.RawCall("eth_sendRawTransaction", encodedTransactionStr)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		p.logger.Error().Err(err).Msg("failed update state on L1 request")
	}
	p.seqno.Add(1)
	// TODO send bloob with transactions and KZG proof
	return nil
}
