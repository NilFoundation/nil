package synccommittee

import (
	"bytes"
	"fmt"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

const (
	SepoliaChainId = uint64(0)
	DefaultGas     = uint64(2000)
)

// @component UpdateStateTransaction UpdateStateTransaction object "The transaction for update state on L1."
// @componentprop chainId "The L1 chain id."
// @componentprop data "The data of transaction: proved state root, new state root."
// @componentprop value "The transer value, allways 0."
// @componentprop nonce "The nonce of transaction."
// @componentprop gas "The default gas value."
type UpdateStateTransaction struct {
	ChainId uint64      `json:"chainId"`
	Data    []byte      `json:"data"`
	Value   types.Value `json:"value"`
	Nonce   uint64      `json:"nonce"`
	Gas     uint64      `json:"gas"`
}

type Proposer struct {
	l1EndPoint string
	client     *rpc.Client
	seqno      atomic.Uint64
	logger     zerolog.Logger
}

func NewProposer(endpoint string, logger zerolog.Logger) *Proposer {
	client := rpc.NewClient(endpoint, logger)

	return &Proposer{
		client:     client,
		l1EndPoint: endpoint,
		logger:     logger,
	}
}

func (p *Proposer) createStateUpdateTransaction(provedStateRoot, newStateRoot common.Hash) (*UpdateStateTransaction, error) {
	if provedStateRoot.Empty() || newStateRoot.Empty() {
		return nil, fmt.Errorf("empty hash for state update transaction %d", p.seqno.Load())
	}
	functionSelector := make([]byte, 4)
	functionSelector[0] = 1
	functionSelector[1] = 2
	functionSelector[2] = 3
	functionSelector[3] = 4
	data := bytes.Join([][]byte{functionSelector, provedStateRoot.Bytes(), newStateRoot.Bytes()}, nil)
	return &UpdateStateTransaction{
		ChainId: SepoliaChainId,
		Data:    data,
		Value:   types.NewValueFromUint64(0),
		Nonce:   p.seqno.Load(),
		Gas:     DefaultGas,
	}, nil
}

func (p *Proposer) SendProof(provedStateRoot, newStateRoot common.Hash, transactions []*prunedTransaction) error {
	p.logger.Debug().Int64("seqno", int64(p.seqno.Load())).Int64("transactionsCount", int64(len(transactions))).Msg("skip processing transactions")
	// call UpdateState L1 contract
	_, err := p.client.RawCall("eth_sendRawTransaction", p.seqno.Load(), provedStateRoot, newStateRoot, transactions)
	if err != nil {
		// TODO return err after enable endpoint
		p.logger.Error().Err(err).Msg("failed update state on L1 request")
	}
	p.seqno.Add(1)
	// TODO send bloob with transactions and KZG proof
	return nil
}
