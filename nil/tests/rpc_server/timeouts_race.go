//go:build race

package rpctest

import "time"

const (
	ReceiptWaitTimeout    = time.Minute
	ReceiptPollInterval   = time.Second
	ZeroStateWaitTimeout  = 30 * time.Second
	ZeroStatePollInterval = time.Second
)
