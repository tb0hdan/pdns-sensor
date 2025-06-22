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
	tick := time.NewTicker(60 * time.Second)
	for range tick.C {
		domains := q.Get()
		if len(domains) == 0 {
			continue
		}
		// Here you would typically send the domains to your API or process them further
		s.logger.Info().Msgf("Submitting %d domains: %v\n", len(domains), domains)
		err := s.client.SubmitDomains(domains)
		if err != nil {
			s.logger.Printf("Error submitting domains: %v", err)
		} else {
			s.logger.Info().Msgf("Successfully submitted %d domains.\n", len(domains))
		}
	}
}

func NewSubmitter(client clients.Client, logger zerolog.Logger) *Submitter {
	return &Submitter{
		client: client,
		logger: logger,
	}
}
