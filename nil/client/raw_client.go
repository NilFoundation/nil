package client

import (
	"context"
	"encoding/json"
)

type RawClient interface {
	// RawCall sends a request to the server with the given method and parameters,
	// and returns the response as json.RawMessage, or an error if the call fails
	RawCall(ctx context.Context, method string, params ...any) (json.RawMessage, error)

	// PlainTextCall sends request as is and returns raw output.
	// Function is useful mainly for testing purposes.
	PlainTextCall(ctx context.Context, requestBody []byte) (json.RawMessage, error)
}
