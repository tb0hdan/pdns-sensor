package models

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type MockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]interface{}),
	}
}

func (c *MockCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.data[key]
	return val, ok
}

func (c *MockCache) SetEx(key string, value interface{}, expires int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

type QueueTestSuite struct {
	suite.Suite
	queue *DomainQueue
	cache *MockCache
}

func (suite *QueueTestSuite) SetupTest() {
	suite.cache = NewMockCache()
	suite.queue = NewDomainQueue(suite.cache, 3600)
}

func (suite *QueueTestSuite) TestAddValidDomain() {
	domain := "example.com"
	suite.queue.Add(domain)
	
	suite.Equal(1, suite.queue.Count())
	domains := suite.queue.Get()
	suite.Contains(domains, domain)
}

func (suite *QueueTestSuite) TestAddDuplicateDomain() {
	domain := "example.com"
	suite.queue.Add(domain)
	suite.queue.Add(domain) // Add same domain again
	
	suite.Equal(1, suite.queue.Count())
}

func (suite *QueueTestSuite) TestAddCachedDomain() {
	domain := "cached.com"
	suite.cache.SetEx(domain, true, 3600)
	
	suite.queue.Add(domain)
	suite.Equal(0, suite.queue.Count())
}

func (suite *QueueTestSuite) TestAddInvalidDomain() {
	// Test domains that should be rejected
	invalidDomains := []string{
		"",
		"example",       // Single label
		"test.local",    // .local domains are rejected
		"test.localhost", // .localhost domains are rejected
	}
	
	for _, domain := range invalidDomains {
		suite.queue.Add(domain)
	}
	
	// Check result
	count := suite.queue.Count()
	if count != 0 {
		// Debug: see what was actually added
		domains := suite.queue.Get()
		suite.Failf("Expected 0 domains, got %d: %v", "", count, domains)
	}
}

func (suite *QueueTestSuite) TestAddLowercasesDomain() {
	suite.queue.Add("EXAMPLE.COM")
	suite.queue.Add("Example.Com")
	
	suite.Equal(1, suite.queue.Count())
	domains := suite.queue.Get()
	suite.Contains(domains, "example.com")
}

func (suite *QueueTestSuite) TestGet() {
	domains := []string{"example1.com", "example2.com", "example3.com"}
	for _, d := range domains {
		suite.queue.Add(d)
	}
	
	retrieved := suite.queue.Get()
	suite.ElementsMatch(domains, retrieved)
	
	// Queue should be empty after Get()
	suite.Equal(0, suite.queue.Count())
}

func (suite *QueueTestSuite) TestGetEmptyQueue() {
	domains := suite.queue.Get()
	suite.Empty(domains)
}

func (suite *QueueTestSuite) TestCount() {
	suite.Equal(0, suite.queue.Count())
	
	suite.queue.Add("example1.com")
	suite.Equal(1, suite.queue.Count())
	
	suite.queue.Add("example2.com")
	suite.Equal(2, suite.queue.Count())
	
	suite.queue.Get()
	suite.Equal(0, suite.queue.Count())
}

func (suite *QueueTestSuite) TestConcurrentAdd() {
	var wg sync.WaitGroup
	numGoroutines := 100
	domainsPerGoroutine := 10
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < domainsPerGoroutine; j++ {
				domain := fmt.Sprintf("example%d-%d.com", id, j)
				suite.queue.Add(domain)
			}
		}(i)
	}
	wg.Wait()
	
	// All unique domains should be added
	suite.Equal(numGoroutines*domainsPerGoroutine, suite.queue.Count())
}

func (suite *QueueTestSuite) TestConcurrentGetAndAdd() {
	var wg sync.WaitGroup
	
	// Add domains
	for i := 0; i < 50; i++ {
		suite.queue.Add(fmt.Sprintf("initial%d.com", i))
	}
	
	// Concurrently add and get domains
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			suite.queue.Add(fmt.Sprintf("concurrent%d.com", i))
			time.Sleep(time.Millisecond)
		}
	}()
	
	go func() {
		defer wg.Done()
		totalRetrieved := 0
		for i := 0; i < 10; i++ {
			domains := suite.queue.Get()
			totalRetrieved += len(domains)
			time.Sleep(10 * time.Millisecond)
		}
	}()
	
	wg.Wait()
	
	// No panic should occur during concurrent operations
	suite.True(true)
}

func (suite *QueueTestSuite) TestNewDomainQueue() {
	cache := NewMockCache()
	ttl := int64(7200)
	queue := NewDomainQueue(cache, ttl)
	
	suite.NotNil(queue)
	suite.NotNil(queue.Lock)
	suite.NotNil(queue.Domains)
	suite.Equal(cache, queue.cache)
	suite.Equal(ttl, queue.cacheTTL)
	suite.Equal(0, len(queue.Domains))
}

func TestQueueTestSuite(t *testing.T) {
	suite.Run(t, new(QueueTestSuite))
}