package srv

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	GracefulShutdown bool
}

func DefaultConfig() Config {
	return Config{
		GracefulShutdown: true,
	}
}

type Service struct {
	workers []Worker
	config  Config
	logger  zerolog.Logger
}

func NewService(config Config, logger zerolog.Logger, workers ...Worker) Service {
	return Service{
		workers: workers,
		config:  config,
		logger:  logger,
	}
}

func (s *Service) Run(ctx context.Context) error {
	defer telemetry.Shutdown(ctx)

	if s.config.GracefulShutdown {
		signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer stop()
		ctx = signalCtx
	}

	group, gCtx := errgroup.WithContext(ctx)

	for _, worker := range s.workers {
		group.Go(func() error {
			return s.runWorker(worker, gCtx)
		})
	}

	return group.Wait()
}

func (s *Service) runWorker(worker Worker, ctx context.Context) error {
	name := worker.Name()
	s.logger.Info().Str("worker", name).Msg("starting worker")

	err := worker.Run(ctx)

	var logLevel zerolog.Level
	if err != nil && !errors.Is(err, context.Canceled) {
		logLevel = zerolog.ErrorLevel
	} else {
		logLevel = zerolog.InfoLevel
	}

	s.logger.WithLevel(logLevel).Err(err).Str("worker", name).Msg("worker stopped")
	return err
}

func LoggerWithWorkerName(logger zerolog.Logger, name string) zerolog.Logger {
	return logger.With().Str("worker", name).Logger()
}
