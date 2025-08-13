//go:build !(linux && amd64)

package pcap

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
)

type PCAP struct {
	queue  *models.DomainQueue
	logger zerolog.Logger
}

// Stop is a placeholder for the actual implementation of stopping the PCAP source.
func (p *PCAP) Stop(ctx context.Context) error {
	p.logger.Info().Msg("Stopping PCAP source...")
	return nil
}

func (p *PCAP) Start() error {
	return errors.New("PCAP source is not supported on this platform")
}

func NewPCAP(queue *models.DomainQueue, logger zerolog.Logger) sources.Source {
	return &PCAP{
		queue:  queue,
		logger: logger,
	}
}
