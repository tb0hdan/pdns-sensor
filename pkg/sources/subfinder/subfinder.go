package subfinder

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/projectdiscovery/subfinder/v2/pkg/resolve"
	"github.com/projectdiscovery/subfinder/v2/pkg/runner"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	"github.com/weppos/publicsuffix-go/publicsuffix"
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
				// Extract parent domain
				parentDomain := s.getParentDomain(domain)
				if parentDomain == "" {
					s.logger.Debug().Str("domain", domain).Msg("Could not extract parent domain, skipping")
					continue
				}

				// Log if we extracted a different parent domain
				if parentDomain != domain {
					s.logger.Info().
						Str("original", domain).
						Str("parent", parentDomain).
						Msg("Extracted parent domain from subdomain")
				}

				// Check if we've already processed this parent domain recently
				if s.isRecentlyProcessed(parentDomain) {
					s.logger.Debug().
						Str("parent", parentDomain).
						Msg("Parent domain recently processed, skipping")
					continue
				}

				// Process parent domain with subfinder
				s.discoverSubdomains(ctx, parentDomain)

				// Mark parent domain as processed
				s.markProcessed(parentDomain)

				// Re-add original domain to queue so it stays available for other sources
				s.queue.Add(domain)
			}
		}
	}
}

func (s *Source) discoverSubdomains(ctx context.Context, domain string) {
	s.logger.Info().Str("parent_domain", domain).Msg("Starting subdomain discovery for parent domain")

	// Track discovered subdomains count
	var discoveredCount int

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
			discoveredCount++
			s.logger.Info().
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
		// Enumeration completed - log summary
		s.logger.Info().
			Str("domain", domain).
			Int("discovered", discoveredCount).
			Msg("Subdomain discovery completed")
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

// getParentDomain extracts the parent domain from a given domain
// For example: www.google.com -> google.com, sub.example.co.uk -> example.co.uk
func (s *Source) getParentDomain(domain string) string {
	// Remove any trailing dots
	domain = strings.TrimSuffix(domain, ".")

	// Parse the domain to get the registered domain (eTLD+1)
	parsed, err := publicsuffix.Parse(domain)
	if err != nil {
		s.logger.Debug().Err(err).Str("domain", domain).Msg("Failed to parse domain")
		return ""
	}

	// If there's no SLD (second-level domain), it means we got an invalid domain or just a TLD
	if parsed.SLD == "" {
		return ""
	}

	// Return the registered domain (SLD + TLD)
	// This will be like "google.com" or "example.co.uk"
	return parsed.SLD + "." + parsed.TLD
}

func NewSubfinder(queue *models.DomainQueue, logger zerolog.Logger, cacheTTL int64) sources.Source {
	return &Source{
		queue:          queue,
		logger:         logger,
		processedCache: make(map[string]time.Time),
		cacheTTL:       time.Duration(cacheTTL) * time.Second,
	}
}
