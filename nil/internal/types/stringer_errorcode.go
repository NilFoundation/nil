//go:build stringer
// +build stringer

package types

type ErrorCode int

const (
	ErrorNone ErrorCode = iota
	ErrorBadRequest
	ErrorTimeout
)

