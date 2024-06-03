package client

import "encoding/json"

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	// Call sends a request to the server with the given method and parameters,
	// and returns the response as json.RawMessage, or an error if the call fails
	Call(method string, params []interface{}) (json.RawMessage, error)
}
