package models

import (
	"strings"
	"sync"

	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

type DomainQueue struct {
	Domains []string
	Lock    *sync.Mutex
}

func (q *DomainQueue) Add(domain string) {
	q.Lock.Lock()
	defer q.Lock.Unlock()
	// Fix mangled domain names
	domain = strings.ToLower(domain)
	// Validate the domain before adding it to the queue
	if !utils.IsValidDomain(domain) {
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
	q.Lock.Lock()
	defer q.Lock.Unlock()
	domains := make([]string, len(q.Domains))
	copy(domains, q.Domains)
	q.Domains = nil // Clear the queue after getting the domains
	return domains
}

func (q *DomainQueue) Count() int {
	q.Lock.Lock()
	defer q.Lock.Unlock()
	return len(q.Domains)
}

func NewDomainQueue() *DomainQueue {
	return &DomainQueue{
		Domains: make([]string, 0),
		Lock:    &sync.Mutex{},
	}
}
