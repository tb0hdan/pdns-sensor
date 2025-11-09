package subfinder

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/projectdiscovery/subfinder/v2/pkg/resolve"
	"github.com/projectdiscovery/subfinder/v2/pkg/runner"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
)

type Source struct {
	queue          *models.DomainQueue
	logger         zerolog.Logger
	cancelFunc     context.CancelFunc
	wg             sync.WaitGroup
	processedCache map[string]time.Time
	cacheMutex     sync.RWMutex
	cacheTTL       time.Duration
}

func (s *Source) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	s.logger.Info().Msg("Starting Subfinder source...")

	// Start background goroutine to process domains from queue
	s.wg.Add(1)
	go s.processDomains(ctx)

	// Start cache cleanup goroutine
	s.wg.Add(1)
	go s.cleanupCache(ctx)

	return nil
}

func (s *Source) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping Subfinder source...")

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info().Msg("Subfinder source stopped successfully")
	case <-ctx.Done():
		s.logger.Warn().Msg("Subfinder source stop timeout")
	}

	return nil
}

func (s *Source) processDomains(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get domains from the queue (this clears the queue)
			domains := s.queue.Get()

			for _, domain := range domains {
				// Check if we've already processed this domain recently
				if s.isRecentlyProcessed(domain) {
					continue
				}

				// Process domain with subfinder
				s.discoverSubdomains(ctx, domain)

				// Mark as processed
				s.markProcessed(domain)

				// Re-add domain to queue so it stays available for other sources
				s.queue.Add(domain)
			}
		}
	}
}

func (s *Source) discoverSubdomains(ctx context.Context, domain string) {
	s.logger.Debug().Str("domain", domain).Msg("Discovering subdomains")

	// Create subfinder runner options
	runnerInstance, err := runner.NewRunner(&runner.Options{
		Threads:            10,
		Timeout:            30,
		MaxEnumerationTime: 10,
		Domain:             []string{domain},
		Silent:             true,
		NoColor:            true,
		RemoveWildcard:     true,
		OnlyRecursive:      false,
		Output:             io.Discard, // Use io.Discard to prevent nil pointer issues
		OutputFile:         "",         // No output file
		ResultCallback: func(result *resolve.HostEntry) {
			// Add discovered subdomain to the queue
			s.queue.Add(result.Host)
			s.logger.Debug().
				Str("subdomain", result.Host).
				Str("source", result.Source).
				Str("parent", domain).
				Msg("Discovered subdomain")
		},
	})

	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("Failed to create subfinder runner")
		return
	}

	// Run enumeration with context
	done := make(chan struct{})
	go func() {
		err = runnerInstance.RunEnumeration()
		if err != nil {
			s.logger.Error().Err(err).Str("domain", domain).Msg("Subfinder enumeration failed")
		}
		close(done)
	}()

	// Wait for completion or context cancellation
	select {
	case <-done:
		// Enumeration completed
	case <-ctx.Done():
		// Context cancelled, stop enumeration
		s.logger.Debug().Str("domain", domain).Msg("Subfinder enumeration cancelled")
	}
}

func (s *Source) isRecentlyProcessed(domain string) bool {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	if processedTime, exists := s.processedCache[domain]; exists {
		if time.Since(processedTime) < s.cacheTTL {
			return true
		}
	}
	return false
}

func (s *Source) markProcessed(domain string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.processedCache[domain] = time.Now()
}

func (s *Source) cleanupCache(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cacheMutex.Lock()
			now := time.Now()
			for domain, processedTime := range s.processedCache {
				if now.Sub(processedTime) > s.cacheTTL {
					delete(s.processedCache, domain)
				}
			}
			s.cacheMutex.Unlock()
			s.logger.Debug().Msg("Cleaned up subfinder cache")
		}
	}
}

func NewSubfinder(queue *models.DomainQueue, logger zerolog.Logger, cacheTTL int64) sources.Source {
	return &Source{
		queue:          queue,
		logger:         logger,
		processedCache: make(map[string]time.Time),
		cacheTTL:       time.Duration(cacheTTL) * time.Second,
	}
}
