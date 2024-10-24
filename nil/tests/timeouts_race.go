//go:build race

package tests

import "time"

const (
	ReceiptWaitTimeout    = time.Minute
	ReceiptPollInterval   = time.Second
	ZeroStateWaitTimeout  = 30 * time.Second
	ZeroStatePollInterval = time.Second
)
