//go:build !race

package rpctest

import "time"

const (
	ReceiptWaitTimeout    = 15 * time.Second
	ReceiptPollInterval   = 200 * time.Millisecond
	ZeroStateWaitTimeout  = 10 * time.Second
	ZeroStatePollInterval = 100 * time.Millisecond
)
