package synccommittee

import (
	"fmt"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/rs/zerolog"
)

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

func (p *Proposer) sendProof(provedStateRoot, newStateRoot common.Hash, transactions []*prunedTransaction) error {
	p.logger.Debug().Int64("seqno", int64(p.seqno.Load())).Int64("transactionsCount", int64(len(transactions))).Msg("skip processing transactions")
	// call UpdateState L1 contract
	_, err := p.client.RawCall("eth_sendRawTransaction", p.seqno.Load(), provedStateRoot, newStateRoot, transactions)
	if err != nil {
		return fmt.Errorf("failed update state on L1 request: %w", err)
	}
	p.seqno.Add(1)
	// TODO send bloob with transactions and KZG proof
	return nil
}
