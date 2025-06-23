package models

import (
	"strings"
	"sync"

	"github.com/tb0hdan/pdns-sensor/pkg/utils"
)

type CacheInterface interface {
	Get(key string) (value interface{}, ok bool)
	SetEx(key string, value interface{}, expires int64)
}

type DomainQueue struct {
	Domains  []string
	Lock     *sync.Mutex
	cache    CacheInterface
	cacheTTL int64 // Cache TTL in seconds
}

func (q *DomainQueue) Add(domain string) {
	q.Lock.Lock()
	defer q.Lock.Unlock()
	// Fix mangled domain names
	domain = strings.ToLower(domain)
	if _, ok := q.cache.Get(domain); ok {
		return // Domain already exists in the cache
	}
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
	q.cache.SetEx(domain, true, q.cacheTTL) // Store in cache with TTL
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

func NewDomainQueue(cache CacheInterface, cacheTTL int64) *DomainQueue {
	return &DomainQueue{
		Domains:  make([]string, 0),
		Lock:     &sync.Mutex{},
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}
