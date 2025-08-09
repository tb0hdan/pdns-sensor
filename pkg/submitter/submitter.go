package submitter

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/clients"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
)

type Submitter struct {
	client clients.Client
	logger zerolog.Logger
}

func (s *Submitter) QueueSubmitter(q *models.DomainQueue) {
	const maxBatchSize = 1024
	tick := time.NewTicker(60 * time.Second)
	for range tick.C {
		domains := q.Get()
		if len(domains) == 0 {
			continue
		}
		
		for i := 0; i < len(domains); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(domains) {
				end = len(domains)
			}
			batch := domains[i:end]
			
			s.logger.Info().Msgf("Submitting batch of %d domains (batch %d/%d)\n", len(batch), (i/maxBatchSize)+1, (len(domains)+maxBatchSize-1)/maxBatchSize)
			err := s.client.SubmitDomains(batch)
			if err != nil {
				s.logger.Error().Err(err).Msgf("Error submitting batch of %d domains", len(batch))
			} else {
				s.logger.Info().Msgf("Successfully submitted batch of %d domains.\n", len(batch))
			}
		}
	}
}

func NewSubmitter(client clients.Client, logger zerolog.Logger) *Submitter {
	return &Submitter{
		client: client,
		logger: logger,
	}
}
