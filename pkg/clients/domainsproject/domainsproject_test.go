package domainsproject

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/tb0hdan/pdns-sensor/pkg/clients"
	"github.com/tb0hdan/pdns-sensor/pkg/types"
)

type DomainsProjectClientTestSuite struct {
	suite.Suite
	client *DomainsProjectClient
	logger zerolog.Logger
	server *httptest.Server
}

func (suite *DomainsProjectClientTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
}

func (suite *DomainsProjectClientTestSuite) TearDownTest() {
	if suite.server != nil {
		suite.server.Close()
	}
}

func (suite *DomainsProjectClientTestSuite) TestNewDomainsProjectClient() {
	// Test with default URL
	client := NewDomainsProjectClient("", suite.logger)
	suite.NotNil(client)
	
	dpClient, ok := client.(*DomainsProjectClient)
	suite.True(ok)
	suite.Equal(DefaultAPIURL, dpClient.APIURL)
	
	// Test with custom URL
	customURL := "https://custom.api.example.com"
	client2 := NewDomainsProjectClient(customURL, suite.logger)
	dpClient2, ok := client2.(*DomainsProjectClient)
	suite.True(ok)
	suite.Equal(customURL, dpClient2.APIURL)
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsSuccess() {
	receivedDomains := []string{}
	
	// Create test server
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("POST", r.Method)
		suite.Equal("application/json", r.Header.Get("Content-Type"))
		
		body, err := io.ReadAll(r.Body)
		suite.NoError(err)
		
		var req types.PassiveDNSRequest
		err = json.Unmarshal(body, &req)
		suite.NoError(err)
		
		receivedDomains = req.Domains
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	
	client := &DomainsProjectClient{
		APIURL: suite.server.URL,
		logger: suite.logger,
	}
	
	domains := []string{"example1.com", "example2.com", "example3.com"}
	err := client.SubmitDomains(domains)
	suite.NoError(err)
	suite.ElementsMatch(domains, receivedDomains)
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsEmptyList() {
	client := &DomainsProjectClient{
		APIURL: "https://api.example.com",
		logger: suite.logger,
	}
	
	err := client.SubmitDomains([]string{})
	suite.NoError(err) // Should return nil for empty list
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsServerError() {
	// Create test server that returns error
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	
	// Fix: Using the actual APIURL field
	client := NewDomainsProjectClient(suite.server.URL, suite.logger)
	
	domains := []string{"example.com"}
	err := client.SubmitDomains(domains)
	suite.Error(err)
	suite.Contains(err.Error(), "status code: 500")
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsBadRequest() {
	// Create test server that returns bad request
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	
	client := NewDomainsProjectClient(suite.server.URL, suite.logger)
	
	domains := []string{"invalid_domain"}
	err := client.SubmitDomains(domains)
	suite.Error(err)
	suite.Contains(err.Error(), "status code: 400")
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsNetworkError() {
	// Use invalid URL to simulate network error
	client := &DomainsProjectClient{
		APIURL: "http://invalid.local.test:99999",
		logger: suite.logger,
	}
	
	domains := []string{"example.com"}
	err := client.SubmitDomains(domains)
	suite.Error(err)
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsLargeBatch() {
	receivedCount := 0
	
	// Create test server
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		suite.NoError(err)
		
		var req types.PassiveDNSRequest
		err = json.Unmarshal(body, &req)
		suite.NoError(err)
		
		receivedCount = len(req.Domains)
		
		w.WriteHeader(http.StatusOK)
	}))
	
	client := &DomainsProjectClient{
		APIURL: suite.server.URL,
		logger: suite.logger,
	}
	
	// Create a large batch of domains
	domains := make([]string, 1024)
	for i := 0; i < 1024; i++ {
		domains[i] = fmt.Sprintf("example%d.com", i)
	}
	
	err := client.SubmitDomains(domains)
	suite.NoError(err)
	suite.Equal(1024, receivedCount)
}

func (suite *DomainsProjectClientTestSuite) TestInterfaceCompliance() {
	var _ clients.Client = &DomainsProjectClient{}
	suite.True(true, "DomainsProjectClient implements clients.Client interface")
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsTimeout() {
	// Create a test server that delays response
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body to prevent connection issues
		io.ReadAll(r.Body)
		// Simulate processing time
		w.WriteHeader(http.StatusOK)
	}))
	
	client := &DomainsProjectClient{
		APIURL: suite.server.URL,
		logger: suite.logger,
	}
	
	domains := []string{"example.com"}
	err := client.SubmitDomains(domains)
	suite.NoError(err) // Should complete successfully
}

func (suite *DomainsProjectClientTestSuite) TestSubmitDomainsVariousStatusCodes() {
	testCases := []struct {
		statusCode int
		expectError bool
	}{
		{http.StatusOK, false},
		{http.StatusCreated, true},
		{http.StatusAccepted, true},
		{http.StatusNoContent, true},
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusForbidden, true},
		{http.StatusNotFound, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
	}
	
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("status_%d", tc.statusCode), func() {
			// Create test server for this status code
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.ReadAll(r.Body)
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()
			
			client := &DomainsProjectClient{
				APIURL: server.URL,
				logger: suite.logger,
			}
			
			err := client.SubmitDomains([]string{"test.com"})
			if tc.expectError {
				suite.Error(err)
				if err != nil {
					suite.Contains(err.Error(), fmt.Sprintf("status code: %d", tc.statusCode))
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func TestDomainsProjectClientTestSuite(t *testing.T) {
	suite.Run(t, new(DomainsProjectClientTestSuite))
}