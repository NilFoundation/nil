//go:build race

package tests

import "time"

const (
	ReceiptWaitTimeout  = time.Minute
	ReceiptPollInterval = time.Second
	BlockWaitTimeout    = 30 * time.Second
	BlockPollInterval   = time.Second
)
