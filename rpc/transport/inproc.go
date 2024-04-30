package transport

import (
	"context"
	"net"

	"github.com/rs/zerolog"
)

// DialInProc attaches an in-process connection to the given RPC server.
func DialInProc(handler *Server, logger *zerolog.Logger) *Client {
	initctx := context.Background()
	c, _ := newClient(initctx, func(context.Context) (ServerCodec, error) {
		p1, p2 := net.Pipe()
		go handler.ServeCodec(NewCodec(p1))
		return NewCodec(p2), nil
	}, logger)
	return c
}
