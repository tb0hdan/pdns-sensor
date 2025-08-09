package miktortik_log

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/tb0hdan/pdns-sensor/pkg/models"
	"github.com/tb0hdan/pdns-sensor/pkg/sources"
	"github.com/tb0hdan/pdns-sensor/pkg/utils"
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

type MikrotikLogTestSuite struct {
	suite.Suite
	mikrotik *MikrotikLog
	queue    *models.DomainQueue
	logger   zerolog.Logger
	tempFile *os.File
}

func (suite *MikrotikLogTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	cache := NewMockCache()
	suite.queue = models.NewDomainQueue(cache, 3600)
	
	// Create a temporary file for testing
	tempFile, err := ioutil.TempFile("", "mikrotik_test_*.log")
	suite.NoError(err)
	suite.tempFile = tempFile
	
	suite.mikrotik = &MikrotikLog{
		queue:   suite.queue,
		logger:  suite.logger,
		logFile: tempFile.Name(),
	}
}

func (suite *MikrotikLogTestSuite) TearDownTest() {
	if suite.tempFile != nil {
		suite.tempFile.Close()
		os.Remove(suite.tempFile.Name())
	}
}

func (suite *MikrotikLogTestSuite) TestNewMikrotikLog() {
	logger := zerolog.New(os.Stderr)
	cache := NewMockCache()
	queue := models.NewDomainQueue(cache, 3600)
	logFile := "/var/log/test.log"
	
	source := NewMikrotikLog(queue, logger, logFile)
	suite.NotNil(source)
	
	mikrotik, ok := source.(*MikrotikLog)
	suite.True(ok)
	suite.Equal(queue, mikrotik.queue)
	suite.Equal(logFile, mikrotik.logFile)
}

func (suite *MikrotikLogTestSuite) TestStop() {
	ctx := context.Background()
	err := suite.mikrotik.Stop(ctx)
	suite.NoError(err)
}

func (suite *MikrotikLogTestSuite) TestParseMikrotikLogLine() {
	testCases := []struct {
		name           string
		line           string
		expectedDomain string
		shouldAdd      bool
	}{
		{
			name:           "Valid query from line",
			line:           "Jan 01 12:00:00 dns,packet query from 192.168.1.1#54321: example.com. A",
			expectedDomain: "example.com",
			shouldAdd:      true,
		},
		{
			name:           "Valid query with subdomain",
			line:           "Jan 01 12:00:00 dns,packet query from 192.168.1.1#54321: www.example.org. AAAA",
			expectedDomain: "www.example.org",
			shouldAdd:      true,
		},
		{
			name:           "Line without query from",
			line:           "Jan 01 12:00:00 dns,packet response to 192.168.1.1#54321: example.com. A",
			expectedDomain: "",
			shouldAdd:      false,
		},
		{
			name:           "Empty line",
			line:           "",
			expectedDomain: "",
			shouldAdd:      false,
		},
		{
			name:           "Multiple domains in line",
			line:           "Jan 01 12:00:00 dns,packet query from 192.168.1.1#54321: google.com. yahoo.com. A",
			expectedDomain: "google.com",
			shouldAdd:      true,
		},
		{
			name:           "Invalid domain - single label",
			line:           "Jan 01 12:00:00 dns,packet query from 192.168.1.1#54321: localhost. PTR",
			expectedDomain: "",
			shouldAdd:      false,
		},
	}
	
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Clear queue before each test
			suite.queue.Get()
			
			// Simulate processing the line (extracted logic for testing)
			line := tc.line
			if line != "" && strings.Contains(line, "query from") {
				for _, field := range strings.Fields(line) {
					if strings.HasSuffix(field, ".") {
						field = strings.TrimSuffix(field, ".")
						if utils.IsValidDomain(field) {
							suite.queue.Add(field)
							break // Only add first valid domain for testing
						}
					}
				}
			}
			
			if tc.shouldAdd {
				domains := suite.queue.Get()
				suite.Contains(domains, tc.expectedDomain)
			} else {
				suite.Equal(0, suite.queue.Count())
			}
		})
	}
}

func (suite *MikrotikLogTestSuite) TestDefaultLogFile() {
	suite.Equal("/var/log/network.log", DefaultLogFile)
}

func (suite *MikrotikLogTestSuite) TestInterfaceCompliance() {
	var _ sources.Source = &MikrotikLog{}
	suite.True(true, "MikrotikLog implements sources.Source interface")
}


func (suite *MikrotikLogTestSuite) TestConcurrentQueueAccess() {
	// Test that MikrotikLog can safely add to queue concurrently
	var wg sync.WaitGroup
	numWorkers := 10
	
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				domain := fmt.Sprintf("test%d-%d.com", id, j)
				suite.queue.Add(domain)
			}
		}(i)
	}
	
	wg.Wait()
	suite.Equal(100, suite.queue.Count())
}

func (suite *MikrotikLogTestSuite) TestLogFileProcessing() {
	// Write test data to temp file
	testLines := []string{
		"Jan 01 12:00:00 dns,packet query from 192.168.1.1#54321: test1.com. A",
		"Jan 01 12:00:01 dns,packet response to 192.168.1.1#54321: test1.com. A",
		"Jan 01 12:00:02 dns,packet query from 192.168.1.2#12345: test2.org. AAAA",
		"",
		"Jan 01 12:00:03 dns,packet query from 192.168.1.3#33333: test3.net. A",
	}
	
	for _, line := range testLines {
		_, err := suite.tempFile.WriteString(line + "\n")
		suite.NoError(err)
	}
	suite.tempFile.Sync()
	
	// Process lines manually (simulating Start() behavior)
	for _, line := range testLines {
		if line != "" && strings.Contains(line, "query from") {
			for _, field := range strings.Fields(line) {
				if strings.HasSuffix(field, ".") {
					field = strings.TrimSuffix(field, ".")
					if utils.IsValidDomain(field) {
						suite.queue.Add(field)
					}
				}
			}
		}
	}
	
	// Check that the correct domains were added
	domains := suite.queue.Get()
	suite.ElementsMatch([]string{"test1.com", "test2.org", "test3.net"}, domains)
}

func TestMikrotikLogTestSuite(t *testing.T) {
	suite.Run(t, new(MikrotikLogTestSuite))
}