package jsonrpc

import "errors"

// ErrMessageDiscarded is returned when the message is discarded, along with the reason.
var ErrMessageDiscarded = errors.New("message discarded")
