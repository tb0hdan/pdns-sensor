package tcpdump

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

type TCPDump struct {
	queue  *models.DomainQueue
	logger zerolog.Logger
}

// Stop is a placeholder for the actual implementation of stopping the TCPDump source.
func (t *TCPDump) Stop(ctx context.Context) error {
	t.logger.Info().Msg("Stopping TCPDump source...")
	return nil
}

func (t *TCPDump) Start() error {
	args := []string{"-ni", "any", "port", "53"}
	cmd := exec.Command("tcpdump", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.logger.Fatal().Msgf("Error creating StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.logger.Fatal().Msgf("Error starting command: %v", err)
	}
	// We're good to continue, now we can read from stdout
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Filter non-empty lines that don't contain "IP"
		if !strings.Contains(line, "IP") {
			continue
		}
		// Filter lines that do not contain "A?" or "AAAA?"
		if !strings.Contains(line, "A?") && !strings.Contains(line, "AAAA?") {
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
			t.queue.Add(field)
		}
	}

	if err := scanner.Err(); err != nil {
		t.logger.Fatal().Msgf("Error reading stdout: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.logger.Fatal().Msgf("Command finished with error: %v", err)
	}

	t.logger.Info().Msg("Subprocess finished successfully.")
	return nil
}

func NewTCPDump(queue *models.DomainQueue, logger zerolog.Logger) sources.Source {
	return &TCPDump{
		queue:  queue,
		logger: logger,
	}
}
