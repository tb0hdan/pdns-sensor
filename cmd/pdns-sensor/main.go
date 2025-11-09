package main

import (
	_ "embed"
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/memcache"
	"github.com/tb0hdan/pdns-sensor/pkg/clients/domainsproject"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	miktortik_log "github.com/tb0hdan/pdns-sensor/pkg/sources/miktortik-log"
	"github.com/tb0hdan/pdns-sensor/pkg/sources/pcap"
	"github.com/tb0hdan/pdns-sensor/pkg/sources/subfinder"
	"github.com/tb0hdan/pdns-sensor/pkg/sources/tcpdump"
	"github.com/tb0hdan/pdns-sensor/pkg/submitter"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

//go:embed VERSION
var Version string

func main() {
	var (
		debug           = flag.Bool("debug", false, "Enable debug logging")
		enableMikrotik  = flag.Bool("enable-mikrotik", false, "Enable Mikrotik log source")
		enableTCPDump   = flag.Bool("enable-tcpdump", false, "Enable TCPDump source")
		enablePCAP      = flag.Bool("enable-pcap", false, "Enable PCAP source")
		enableSubfinder = flag.Bool("enable-subfinder", false, "Enable Subfinder source for subdomain discovery")
		mikrotikLogFile = flag.String("mikrotik-log-file", miktortik_log.DefaultLogFile, "Path to the Mikrotik log file")
		cacheTTL        = flag.Int64("cache-ttl", 3600, "Cache TTL in seconds (default: 3600 seconds)")
		version         = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()
	if *version {
		println("pdns-sensor version:", Version)
		os.Exit(0)
	}
	if !*enableMikrotik && !*enableTCPDump && !*enablePCAP && !*enableSubfinder {
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
	wrapLogger := utils.WrapLogger(logger)
	cache := memcache.New(wrapLogger)
	queue := models.NewDomainQueue(cache, *cacheTTL)
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
	// If PCAP is enabled, create a new PCAP source
	pcapSource := pcap.NewPCAP(queue, logger)
	if *enablePCAP {
		go func() {
			if err := pcapSource.Start(); err != nil {
				logger.Fatal().Err(err).Msg("Failed to start PCAP source")
			}
		}()
	}

	// If Subfinder is enabled, create a new Subfinder source
	subfinderSource := subfinder.NewSubfinder(queue, logger, *cacheTTL)
	if *enableSubfinder {
		go func() {
			if err := subfinderSource.Start(); err != nil {
				logger.Fatal().Err(err).Msg("Failed to start Subfinder source")
			}
		}()
	}

	// Run the main loop
	utils.Run(logger, []sources.Source{dumper, newMikrotik, pcapSource, subfinderSource})
}
