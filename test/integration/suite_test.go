//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

// testContext holds state shared across step definitions within a scenario.
type testContext struct {
	baseURL      string
	client       *http.Client
	response     *http.Response
	responseBody []byte
	err          error
}

// newTestContext creates a new test context with sensible defaults.
func newTestContext() *testContext {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &testContext{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// reset clears response state between scenarios.
func (tc *testContext) reset() {
	if tc.response != nil && tc.response.Body != nil {
		tc.response.Body.Close()
	}
	tc.response = nil
	tc.responseBody = nil
	tc.err = nil
}

// InitializeScenario registers step definitions for each scenario.
func InitializeScenario(ctx *godog.ScenarioContext) {
	tc := newTestContext()

	// Reset state before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		tc.reset()
		return ctx, nil
	})

	// Clean up after each scenario
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		tc.reset()
		return ctx, nil
	})

	// Register step definitions
	ctx.Step(`^the service is running$`, tc.theServiceIsRunning)
	ctx.Step(`^I request GET "([^"]*)"$`, tc.iRequestGET)
	ctx.Step(`^the response status should be (\d+)$`, tc.theResponseStatusShouldBe)
	ctx.Step(`^the response should contain "([^"]*)"$`, tc.theResponseShouldContain)
}

// theServiceIsRunning verifies the service is reachable.
func (tc *testContext) theServiceIsRunning() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.baseURL+"/-/live", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("service is not running at %s: %w", tc.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// iRequestGET makes a GET request to the specified path.
func (tc *testContext) iRequestGET(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := tc.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	tc.response, tc.err = tc.client.Do(req)
	if tc.err != nil {
		return fmt.Errorf("request failed: %w", tc.err)
	}

	tc.responseBody, tc.err = io.ReadAll(tc.response.Body)
	if tc.err != nil {
		return fmt.Errorf("failed to read response body: %w", tc.err)
	}

	return nil
}

// theResponseStatusShouldBe asserts the response status code.
func (tc *testContext) theResponseStatusShouldBe(expectedCode int) error {
	if tc.response == nil {
		return fmt.Errorf("no response received")
	}

	if tc.response.StatusCode != expectedCode {
		return fmt.Errorf("expected status %d, got %d. Body: %s",
			expectedCode, tc.response.StatusCode, string(tc.responseBody))
	}

	return nil
}

// theResponseShouldContain asserts the response body contains the given text.
func (tc *testContext) theResponseShouldContain(text string) error {
	if tc.responseBody == nil {
		return fmt.Errorf("no response body")
	}

	body := string(tc.responseBody)
	if !contains(body, text) {
		return fmt.Errorf("response body does not contain %q.\nBody: %s", text, body)
	}

	return nil
}

// contains checks if haystack contains needle (simple substring match).
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && findSubstring(haystack, needle)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFeatures runs the GoDog BDD test suite.
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			TestingT: t,
			Tags:     os.Getenv("GODOG_TAGS"),
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
