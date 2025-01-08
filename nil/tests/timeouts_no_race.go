//go:build !race

package tests

import "time"

const (
	ReceiptWaitTimeout  = 15 * time.Second
	ReceiptPollInterval = 250 * time.Millisecond
	BlockWaitTimeout    = 10 * time.Second
	BlockPollInterval   = 100 * time.Millisecond
)
