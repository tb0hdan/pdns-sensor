package domainsproject

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/tb0hdan/pdns-sensor/pkg/clients"
	"github.com/tb0hdan/pdns-sensor/pkg/types"
)

const (
	DefaultAPIURL = "https://api.domainsproject.org/api/ua/passive_dns"
)

type DomainsProjectClient struct {
	APIURL string
	logger zerolog.Logger
}

func (d *DomainsProjectClient) SubmitDomains(domains []string) error {
	if len(domains) == 0 {
		return nil // Nothing to submit
	}

	payload := types.PassiveDNSRequest{Domains: domains}
	// Convert to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal domains to JSON: %w", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), "POST", DefaultAPIURL, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to submit domains, status code: %d", resp.StatusCode)
	}

	return nil
}

func NewDomainsProjectClient(apiURL string, logger zerolog.Logger) clients.Client {
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &DomainsProjectClient{
		APIURL: apiURL,
		logger: logger,
	}
}
