package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIURL = "https://api.domainsproject.org/api/ua/passive_dns"
)

type PassiveDNSRequest struct {
	Domains []string `json:"domains"`
}

func IsDomain(domain string) bool {
	reg := regexp.MustCompile(`(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]`)
	return reg.MatchString(domain)
}

func IsValidDomain(domain string) bool {
	// Sanity checks for domain length and structure
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}
	//
	if strings.HasSuffix(domain, ".local") || strings.HasSuffix(domain, ".localhost") {
		return false
	}
	// Check for dot
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	//
	return IsDomain(domain)
}

type DomainQueue struct {
	Domains []string
	lock    *sync.Mutex
}

func (q *DomainQueue) Add(domain string) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if !IsValidDomain(domain) {
		return
	}
	for _, d := range q.Domains {
		if d == domain {
			return // Domain already exists in the queue
		}
	}
	q.Domains = append(q.Domains, domain)
}

func (q *DomainQueue) Get() []string {
	q.lock.Lock()
	defer q.lock.Unlock()
	domains := make([]string, len(q.Domains))
	copy(domains, q.Domains)
	q.Domains = nil // Clear the queue after getting the domains
	return domains
}

func (q *DomainQueue) Count() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.Domains)
}

func SubmitDomains(domains []string) error {
	if len(domains) == 0 {
		return nil // Nothing to submit
	}

	payload := PassiveDNSRequest{Domains: domains}
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

func QueueSubmitter(q *DomainQueue) {
	tick := time.NewTicker(60 * time.Second)
	for range tick.C {
		domains := q.Get()
		if len(domains) == 0 {
			continue
		}
		// Here you would typically send the domains to your API or process them further
		fmt.Printf("Submitting %d domains: %v\n", len(domains), domains)
		err := SubmitDomains(domains)
		if err != nil {
			log.Printf("Error submitting domains: %v", err)
		} else {
			fmt.Printf("Successfully submitted %d domains.\n", len(domains))
		}
	}
}

func main() {
	args := []string{"-ni", "any", "port", "53"} // Example arguments for tcpdump
	cmd := exec.Command("tcpdump", args...)      // Example: list files in the current directory

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Error creating StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Error starting command: %v", err)
	}
	// We're good to continue, now we can read from stdout
	queue := &DomainQueue{
		Domains: make([]string, 0),
		lock:    &sync.Mutex{},
	}
	go QueueSubmitter(queue)
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
			if !IsValidDomain(field) {
				continue
			}
			queue.Add(field)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading stdout: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalf("Command finished with error: %v", err)
	}

	fmt.Println("Subprocess finished successfully.")
}
