package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/clients/domainsproject"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources/tcpdump"
	"github.com/tb0hdan/pdns-sensor/pkg/submitter"
)

func main() {
	var (
		debug = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()
	// Initialize the logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	queue := models.NewDomainQueue()
	// Initialize the queue newSubmitter
	client := domainsproject.NewDomainsProjectClient("", logger) // Use default API URL
	newSubmitter := submitter.NewSubmitter(client, logger)
	// Start the queue newSubmitter in a separate goroutine
	go newSubmitter.QueueSubmitter(queue)
	dumper := tcpdump.NewTCPDump(queue, logger)
	if err := dumper.Start(); err != nil {
		panic(err)
	}
}
