package concurrent

import (
	"context"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
)

// OnSignal calls the provided function when one of the expected signals is received and returns.
// If the context is canceled, OnSignal returns without calling the function.
//
// For graceful termination, create the main context cancelable
// and run OnSignal with that context and its cancel function.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go OnSignal(ctx, cancel, syscall.SIGINT, syscall.SIGTERM)()
func OnSignal(ctx context.Context, f func(), sigs ...os.Signal) {
	log.Info().Msg("Registering signal handler...")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)

	select {
	case sig := <-ch:
		log.Warn().Msgf("App caught signal %s; calling handler...", sig)
		f()
	case <-ctx.Done():
		log.Warn().Msgf("Signal thread is terminating due to canceled context: %q", ctx.Err())
	}
}
