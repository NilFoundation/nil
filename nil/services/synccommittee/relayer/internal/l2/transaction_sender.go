package l2

import (
	"context"
	"errors"
)

type TransactionSender struct {
	// TODO poll ready events from finality ensurer || ticker
	// Sign event with key (add key provider)
	// Publish event to L2BridgeMessenger contract
	// Drop event from L2 event storage
}

func (ts *TransactionSender) Name() string {
	return "l2-transaction-sender"
}

func (ts *TransactionSender) Run(ctx context.Context, started chan<- struct{}) error {
	return errors.New("not implemented")
}
