package miktortik_log

import (
	"context"
	"strings"

	"github.com/hpcloud/tail"
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

func (m *MikrotikLog) Start() error {
	m.logger.Info().Msg("Starting Mikrotik log source...")
	t, err := tail.TailFile(m.logFile, tail.Config{Follow: true})
	if err != nil {
		m.logger.Error().Err(err).Msgf("Error opening log file: %s", m.logFile)
		return err
	}

	for lineItem := range t.Lines {
		line := strings.TrimSpace(lineItem.Text)
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

func (m *MikrotikLog) Stop(ctx context.Context) error {
	m.logger.Info().Msg("Stopping Mikrotik log source...")
	return nil
}

func NewMikrotikLog(queue *models.DomainQueue, logger zerolog.Logger, logFile string) sources.Source {
	return &MikrotikLog{
		queue:   queue,
		logger:  logger,
		logFile: logFile,
	}
}
