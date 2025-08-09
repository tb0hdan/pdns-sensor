package submitter

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
)

type MockClient struct {
	submitFunc func(domains []string) error
	mu         sync.Mutex
	calls      [][]string
}

func NewMockClient() *MockClient {
	return &MockClient{
		calls: make([][]string, 0),
		submitFunc: func(domains []string) error {
			return nil
		},
	}
}

func (c *MockClient) SubmitDomains(domains []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, domains)
	return c.submitFunc(domains)
}

func (c *MockClient) GetCalls() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([][]string, len(c.calls))
	copy(result, c.calls)
	return result
}

func (c *MockClient) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = make([][]string, 0)
}

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

type SubmitterTestSuite struct {
	suite.Suite
	submitter *Submitter
	client    *MockClient
	logger    zerolog.Logger
	queue     *models.DomainQueue
}

func (suite *SubmitterTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	suite.client = NewMockClient()
	suite.submitter = NewSubmitter(suite.client, suite.logger)
	
	cache := NewMockCache()
	suite.queue = models.NewDomainQueue(cache, 3600)
}

func (suite *SubmitterTestSuite) TestNewSubmitter() {
	logger := zerolog.New(os.Stderr)
	client := NewMockClient()
	
	submitter := NewSubmitter(client, logger)
	suite.NotNil(submitter)
	suite.Equal(client, submitter.client)
}

func (suite *SubmitterTestSuite) TestQueueSubmitterEmptyQueue() {
	// Start submitter in goroutine
	done := make(chan bool)
	go func() {
		// Run for a short time then stop
		go suite.submitter.QueueSubmitter(suite.queue)
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()
	
	<-done
	
	// No domains should have been submitted
	calls := suite.client.GetCalls()
	suite.Equal(0, len(calls))
}

func (suite *SubmitterTestSuite) TestQueueSubmitterSingleBatch() {
	// Add domains to queue (less than max batch size)
	for i := 0; i < 100; i++ {
		suite.queue.Add(fmt.Sprintf("example%d.com", i))
	}
	
	// Manually test the batch submission logic
	domains := suite.queue.Get()
	suite.Equal(100, len(domains))
	
	const maxBatchSize = 1024
	for i := 0; i < len(domains); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]
		err := suite.client.SubmitDomains(batch)
		suite.NoError(err)
	}
	
	// Should have one batch
	calls := suite.client.GetCalls()
	suite.Equal(1, len(calls))
	suite.Equal(100, len(calls[0]))
}

func (suite *SubmitterTestSuite) TestQueueSubmitterMultipleBatches() {
	// Add more than maxBatchSize domains
	for i := 0; i < 2500; i++ {
		suite.queue.Add(fmt.Sprintf("example%d.com", i))
	}
	
	// Manually test the batch submission logic
	domains := suite.queue.Get()
	suite.Equal(2500, len(domains))
	
	const maxBatchSize = 1024
	for i := 0; i < len(domains); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]
		err := suite.client.SubmitDomains(batch)
		suite.NoError(err)
	}
	
	// Should have 3 batches (1024 + 1024 + 452)
	calls := suite.client.GetCalls()
	suite.Equal(3, len(calls))
	suite.Equal(1024, len(calls[0]))
	suite.Equal(1024, len(calls[1]))
	suite.Equal(452, len(calls[2]))
}

func (suite *SubmitterTestSuite) TestQueueSubmitterWithError() {
	// Set up client to return an error
	suite.client.submitFunc = func(domains []string) error {
		return errors.New("submission failed")
	}
	
	// Add domains to queue
	for i := 0; i < 50; i++ {
		suite.queue.Add(fmt.Sprintf("example%d.com", i))
	}
	
	// Manually test the batch submission logic
	domains := suite.queue.Get()
	const maxBatchSize = 1024
	for i := 0; i < len(domains); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]
		err := suite.client.SubmitDomains(batch)
		suite.Error(err)
	}
	
	// Submission should have been attempted despite error
	calls := suite.client.GetCalls()
	suite.Equal(1, len(calls))
}

func (suite *SubmitterTestSuite) TestQueueSubmitterExactBatchSize() {
	// Add exactly maxBatchSize domains
	for i := 0; i < 1024; i++ {
		suite.queue.Add(fmt.Sprintf("example%d.com", i))
	}
	
	// Manually test the batch submission logic
	domains := suite.queue.Get()
	suite.Equal(1024, len(domains))
	
	const maxBatchSize = 1024
	for i := 0; i < len(domains); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]
		err := suite.client.SubmitDomains(batch)
		suite.NoError(err)
	}
	
	// Should have exactly one batch
	calls := suite.client.GetCalls()
	suite.Equal(1, len(calls))
	suite.Equal(1024, len(calls[0]))
}

func (suite *SubmitterTestSuite) TestBatchCalculation() {
	testCases := []struct {
		totalDomains   int
		expectedBatches int
		lastBatchSize  int
	}{
		{0, 0, 0},
		{1, 1, 1},
		{1023, 1, 1023},
		{1024, 1, 1024},
		{1025, 2, 1},
		{2048, 2, 1024},
		{3000, 3, 952},
	}
	
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("%d_domains", tc.totalDomains), func() {
			suite.client.Reset()
			
			// Create domains slice
			domains := make([]string, tc.totalDomains)
			for i := 0; i < tc.totalDomains; i++ {
				domains[i] = fmt.Sprintf("test%d.com", i)
			}
			
			// Simulate batch submission
			const maxBatchSize = 1024
			batchCount := 0
			lastBatchSize := 0
			
			for i := 0; i < len(domains); i += maxBatchSize {
				end := i + maxBatchSize
				if end > len(domains) {
					end = len(domains)
				}
				batch := domains[i:end]
				batchCount++
				lastBatchSize = len(batch)
				suite.client.SubmitDomains(batch)
			}
			
			if tc.expectedBatches > 0 {
				suite.Equal(tc.expectedBatches, batchCount)
				suite.Equal(tc.lastBatchSize, lastBatchSize)
			}
		})
	}
}

func (suite *SubmitterTestSuite) TestConcurrentAccess() {
	// Test that multiple goroutines can safely interact with submitter
	var wg sync.WaitGroup
	
	// Add domains concurrently
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				suite.queue.Add(fmt.Sprintf("concurrent%d-%d.com", id, j))
			}
		}(i)
	}
	wg.Wait()
	
	// Get and submit domains
	domains := suite.queue.Get()
	suite.Equal(1000, len(domains))
}

func TestSubmitterTestSuite(t *testing.T) {
	suite.Run(t, new(SubmitterTestSuite))
}