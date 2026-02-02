# Writing Integration Tests

This guide covers writing BDD-style integration tests using GoDog (Cucumber for Go).

## Overview

Integration tests verify the service's HTTP endpoints against a running instance.
They use Gherkin syntax for human-readable specifications.

**Framework:** [cucumber/godog](https://github.com/cucumber/godog)

---

## Directory Structure

```text
test/
├── features/           # Gherkin feature files
│   └── health.feature
└── integration/        # Go test code
    └── suite_test.go   # Step definitions and test runner
```

---

## Gherkin Feature Files

Feature files describe behavior in plain English.

**File:** `test/features/health.feature`

```gherkin
@smoke @health
Feature: Health Endpoints
  As a Kubernetes operator
  I want health endpoints
  So that I can monitor service health and make routing decisions

  Background:
    Given the service is running

  @smoke @liveness
  Scenario: Liveness probe returns OK
    When I request GET "/-/live"
    Then the response status should be 200
    And the response should contain "status"
    And the response should contain "ok"

  @readiness @requires-network
  Scenario: Readiness probe returns OK when healthy
    When I request GET "/-/ready"
    Then the response status should be 200
    And the response should contain "status"

  @smoke @build
  Scenario: Build info returns version information
    When I request GET "/-/build"
    Then the response status should be 200
    And the response should contain "version"
    And the response should contain "commit"
```

### Gherkin Syntax

| Keyword      | Purpose                                       |
| ------------ | --------------------------------------------- |
| `Feature`    | Describes the feature being tested            |
| `Background` | Steps run before each scenario                |
| `Scenario`   | A single test case                            |
| `Given`      | Preconditions/setup                           |
| `When`       | Action being tested                           |
| `Then`       | Expected outcome                              |
| `And`        | Additional steps (continues previous keyword) |
| `@tag`       | Tags for filtering tests                      |

---

## Step Definitions

Step definitions map Gherkin steps to Go code.

**File:** `test/integration/suite_test.go`

```go
//go:build integration

package integration

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
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
    ctx.Step(`^I request POST "([^"]*)" with body:$`, tc.iRequestPOSTWithBody)
    ctx.Step(`^the response status should be (\d+)$`, tc.theResponseStatusShouldBe)
    ctx.Step(`^the response should contain "([^"]*)"$`, tc.theResponseShouldContain)
    ctx.Step(`^the response header "([^"]*)" should be "([^"]*)"$`, tc.theResponseHeaderShouldBe)
}

// Step: the service is running
func (tc *testContext) theServiceIsRunning() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.baseURL+"/-/live", nil)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }

    resp, err := tc.client.Do(req)
    if err != nil {
        return fmt.Errorf("service not running at %s: %w", tc.baseURL, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: status %d", resp.StatusCode)
    }

    return nil
}

// Step: I request GET "{path}"
func (tc *testContext) iRequestGET(path string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.baseURL+path, nil)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }

    tc.response, tc.err = tc.client.Do(req)
    if tc.err != nil {
        return fmt.Errorf("request failed: %w", tc.err)
    }

    tc.responseBody, tc.err = io.ReadAll(tc.response.Body)
    if tc.err != nil {
        return fmt.Errorf("reading body: %w", tc.err)
    }

    return nil
}

// Step: I request POST "{path}" with body:
func (tc *testContext) iRequestPOSTWithBody(path string, body *godog.DocString) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(
        ctx,
        http.MethodPost,
        tc.baseURL+path,
        strings.NewReader(body.Content),
    )
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    tc.response, tc.err = tc.client.Do(req)
    if tc.err != nil {
        return fmt.Errorf("request failed: %w", tc.err)
    }

    tc.responseBody, tc.err = io.ReadAll(tc.response.Body)
    return tc.err
}

// Step: the response status should be {code}
func (tc *testContext) theResponseStatusShouldBe(expected int) error {
    if tc.response == nil {
        return fmt.Errorf("no response received")
    }

    if tc.response.StatusCode != expected {
        return fmt.Errorf("expected status %d, got %d\nBody: %s",
            expected, tc.response.StatusCode, string(tc.responseBody))
    }

    return nil
}

// Step: the response should contain "{text}"
func (tc *testContext) theResponseShouldContain(text string) error {
    if tc.responseBody == nil {
        return fmt.Errorf("no response body")
    }

    if !strings.Contains(string(tc.responseBody), text) {
        return fmt.Errorf("body does not contain %q\nBody: %s", text, string(tc.responseBody))
    }

    return nil
}

// Step: the response header "{name}" should be "{value}"
func (tc *testContext) theResponseHeaderShouldBe(name, expected string) error {
    if tc.response == nil {
        return fmt.Errorf("no response received")
    }

    actual := tc.response.Header.Get(name)
    if actual != expected {
        return fmt.Errorf("header %q: expected %q, got %q", name, expected, actual)
    }

    return nil
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
        t.Fatal("integration tests failed")
    }
}
```

---

## Adding New Scenarios

### 1. Write the Feature

**File:** `test/features/quotes.feature`

```gherkin
@quotes
Feature: Quote API
  As an API consumer
  I want to fetch quotes
  So that I can display inspirational content

  Background:
    Given the service is running

  @smoke
  Scenario: Get random quote
    When I request GET "/api/v1/quotes/random"
    Then the response status should be 200
    And the response should contain "content"
    And the response should contain "author"

  Scenario: Get quote by ID returns 404 for invalid ID
    When I request GET "/api/v1/quotes/invalid-id"
    Then the response status should be 404
    And the response should contain "NOT_FOUND"
```

### 2. Add Step Definitions (if needed)

If your scenarios need new steps, add them to `suite_test.go`:

```go
func InitializeScenario(ctx *godog.ScenarioContext) {
    // ... existing setup ...

    // Add new steps
    ctx.Step(`^I set header "([^"]*)" to "([^"]*)"$`, tc.iSetHeader)
    ctx.Step(`^the response JSON "([^"]*)" should equal "([^"]*)"$`, tc.theResponseJSONShouldEqual)
}

func (tc *testContext) iSetHeader(name, value string) error {
    if tc.headers == nil {
        tc.headers = make(map[string]string)
    }
    tc.headers[name] = value
    return nil
}

func (tc *testContext) theResponseJSONShouldEqual(path, expected string) error {
    // Use gjson or similar for JSON path queries
    actual := gjson.GetBytes(tc.responseBody, path).String()
    if actual != expected {
        return fmt.Errorf("JSON path %q: expected %q, got %q", path, expected, actual)
    }
    return nil
}
```

---

## Running Integration Tests

### Prerequisites

Start the service first:

```bash
# Terminal 1: Run the service
task run
```

### Run Tests

```bash
# Terminal 2: Run integration tests
task test:integration

# Or with go test directly
go test -tags integration -v ./test/integration/...

# Run specific tags
GODOG_TAGS="@smoke" task test:integration
GODOG_TAGS="@health && ~@requires-network" task test:integration

# Against different environment
BASE_URL=http://staging:8080 task test:integration
```

### Tag Expressions

| Expression             | Meaning                       |
| ---------------------- | ----------------------------- |
| `@smoke`               | Run scenarios tagged `@smoke` |
| `@health && @smoke`    | Run scenarios with both tags  |
| `@health \|\| @quotes` | Run scenarios with either tag |
| `~@requires-network`   | Exclude scenarios with tag    |
| `@smoke && ~@slow`     | Smoke tests except slow ones  |

---

## Test Context Pattern

The `testContext` struct holds state across steps within a scenario:

```go
type testContext struct {
    // Configuration
    baseURL string
    client  *http.Client
    headers map[string]string

    // Request state
    requestBody []byte

    // Response state
    response     *http.Response
    responseBody []byte
    err          error

    // Business state (for multi-step scenarios)
    createdID string
    authToken string
}
```

### State Management

- **Reset between scenarios:** Use `ctx.Before()` hook
- **Clean up resources:** Use `ctx.After()` hook
- **Share state between steps:** Use `testContext` fields

---

## Best Practices

### 1. Use Tags for Organization

```gherkin
@smoke @critical       # Critical path tests
@regression            # Full regression suite
@requires-network      # Tests that need external services
@slow                  # Long-running tests
```

### 2. Keep Scenarios Independent

Each scenario should be self-contained:

```gherkin
# Good: Independent scenario
Scenario: Create and retrieve user
  Given I create a user with email "test@example.com"
  When I request GET "/users/{lastCreatedID}"
  Then the response status should be 200

# Bad: Depends on previous scenario
Scenario: Retrieve user
  When I request GET "/users/123"  # Assumes user exists
  Then the response status should be 200
```

### 3. Use Background for Common Setup

```gherkin
Background:
  Given the service is running
  And I am authenticated as "test-user"
```

### 4. Descriptive Error Messages

```go
func (tc *testContext) theResponseStatusShouldBe(expected int) error {
    if tc.response.StatusCode != expected {
        // Include helpful debugging info
        return fmt.Errorf(
            "expected status %d, got %d\nURL: %s\nBody: %s",
            expected, tc.response.StatusCode,
            tc.response.Request.URL,
            string(tc.responseBody),
        )
    }
    return nil
}
```

---

## Checklist

- [ ] Feature file in `test/features/`
- [ ] Appropriate tags for filtering
- [ ] Step definitions in `suite_test.go`
- [ ] Build tag `//go:build integration`
- [ ] Context reset in `Before` hook
- [ ] Resource cleanup in `After` hook
- [ ] Test runs against local service

---

## Related Documentation

- [Writing Unit Tests](./writing-unit-tests.md)
- [Writing Benchmark Tests](./writing-benchmark-tests.md)
- [GoDog Documentation](https://github.com/cucumber/godog)
