package miktortik_log

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

const (
	DefaultLogFile = "/var/log/network.log"
)

type MikrotikLog struct {
	queue   *models.DomainQueue
	logger  zerolog.Logger
	logFile string
}

func (m *MikrotikLog) Stop(ctx context.Context) error {
	// This is a placeholder for the actual implementation of stopping the Mikrotik log source.
	// The implementation would typically involve closing any connections to the Mikrotik router
	// and cleaning up resources.
	m.logger.Info().Msg("Stopping Mikrotik log source...")
	f, err := os.Open(m.logFile)
	if err != nil {
		m.logger.Error().Err(err).Msgf("Error opening log file: %s", m.logFile)
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			m.logger.Error().Err(err).Msg("Error closing log file")
		}
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Filter non-empty lines that don't contain "IP"
		if !strings.Contains(line, "query from") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if !strings.HasSuffix(field, ".") {
				continue
			}
			field = strings.TrimSuffix(field, ".")
			if !utils.IsValidDomain(field) {
				continue
			}
			m.queue.Add(field)
		}

	}

	return nil
}

func (m *MikrotikLog) Start() error {
	m.logger.Info().Msg("Starting Mikrotik log source...")
	if m.logFile == "" {
		m.logFile = DefaultLogFile
	}
	// Here you would add the logic to connect to the Mikrotik router and read logs.
	return nil
}

func NewMikrotikLog(queue *models.DomainQueue, logger zerolog.Logger, logFile string) sources.Source {
	return &MikrotikLog{
		queue:   queue,
		logger:  logger,
		logFile: logFile,
	}
}
