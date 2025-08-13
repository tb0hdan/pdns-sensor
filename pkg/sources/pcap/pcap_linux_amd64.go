//go:build linux && amd64

package pcap

import (
	"context"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

type PCAP struct {
	queue  *models.DomainQueue
	logger zerolog.Logger
}

// Stop is a placeholder for the actual implementation of stopping the PCAP source.
func (p *PCAP) Stop(ctx context.Context) error {
	p.logger.Info().Msg("Stopping PCAP source...")
	return nil
}


func NewPCAP(queue *models.DomainQueue, logger zerolog.Logger) sources.Source {
	return &PCAP{
		queue:  queue,
		logger: logger,
	}
}

func (p *PCAP) Start() error {
	// Open the device for capturing
	handle, err := pcap.OpenLive("any", 1600, true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("error opening device: %w", err)
	}
	defer handle.Close()

	err = handle.SetBPFFilter("port 53 and (udp or tcp)")
	if err != nil {
		return fmt.Errorf("error setting BPF filter: %w", err)
	}

	// Use the handle as a packet source to process all packets
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		dnsLayer := packet.Layer(layers.LayerTypeDNS)
		if dnsLayer == nil {
			continue
		}
		dns, _ := dnsLayer.(*layers.DNS)
		for _, question := range dns.Questions {
			if question.Type != layers.DNSTypeA && question.Type != layers.DNSTypeAAAA {
				continue // Skip non-A and non-AAAA DNS questions
			}
			field := string(question.Name)
			if !utils.IsValidDomain(field) {
				continue
			}
			p.queue.Add(field)
		}

	}
	return nil
}
