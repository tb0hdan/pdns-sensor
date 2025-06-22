package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/clients/domainsproject"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	miktortik_log "github.com/tb0hdan/pdns-sensor/pkg/sources/miktortik-log"
	"github.com/tb0hdan/pdns-sensor/pkg/sources/tcpdump"
	"github.com/tb0hdan/pdns-sensor/pkg/submitter"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

func main() {
	var (
		debug           = flag.Bool("debug", false, "Enable debug logging")
		enableMikrotik  = flag.Bool("enable-mikrotik", false, "Enable Mikrotik log source")
		enableTCPDump   = flag.Bool("enable-tcpdump", false, "Enable TCPDump source")
		mikrotikLogFile = flag.String("mikrotik-log-file", miktortik_log.DefaultLogFile, "Path to the Mikrotik log file")
	)
	flag.Parse()
	if !*enableMikrotik && !*enableTCPDump {
		flag.Usage()
		os.Exit(1)
	}
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
	if *enableTCPDump {
		go func() {
			if err := dumper.Start(); err != nil {
				panic(err)
			}
		}()
	}

	newMikrotik := miktortik_log.NewMikrotikLog(queue, logger, *mikrotikLogFile)
	if *enableMikrotik {
		go func() {
			if err := newMikrotik.Start(); err != nil {
				panic(err)
			}
		}()
	}

	// Run the main loop
	utils.Run(logger, []sources.Source{dumper, newMikrotik})
}
