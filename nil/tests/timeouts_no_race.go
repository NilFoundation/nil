//go:build !race

package tests

import "time"

const (
	ReceiptWaitTimeout    = 15 * time.Minute
	ReceiptPollInterval   = 250 * time.Millisecond
	BlockWaitTimeout      = 10 * time.Minute
	BlockPollInterval     = 100 * time.Millisecond
	ShardTickWaitTimeout  = 30 * time.Second
	ShardTickPollInterval = 1 * time.Second
)
