//go:build !race

package tests

import "time"

const (
	ReceiptWaitTimeout    = 15 * time.Second
	ReceiptPollInterval   = 250 * time.Millisecond
	ZeroStateWaitTimeout  = 10 * time.Second
	ZeroStatePollInterval = 100 * time.Millisecond
)
