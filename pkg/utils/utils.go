package utils

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
)

const (
	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout = 10 * time.Second
)

func Run(logger zerolog.Logger, sourceList []sources.Source) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	for _, s := range sourceList {
		if err := s.Stop(ctx); err != nil {
			logger.Fatal().Err(err).Msgf("Error stopping source %T", s)
		}
	}
}
