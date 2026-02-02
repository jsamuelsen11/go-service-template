# Writing Benchmark Tests

This guide covers writing Go benchmark tests for performance measurement.

## Overview

Benchmark tests measure code performance (execution time, memory allocations). They use Go's built-in `testing` package.

**Location:** `test/benchmark/`

---

## Benchmark Function Signature

```go
func BenchmarkFunctionName(b *testing.B) {
    // Setup (not measured)
    data := prepareTestData()

    b.ResetTimer() // Reset after setup

    // Benchmark loop (measured)
    for i := 0; i < b.N; i++ {
        functionUnderTest(data)
    }
}
```

**Key points:**

- Function name must start with `Benchmark`
- Receives `*testing.B` (not `*testing.T`)
- Loop `b.N` times (Go adjusts N for accurate measurement)
- Use `b.ResetTimer()` after expensive setup

---

## Basic Examples

### Simple Benchmark

```go
package benchmark

import (
    "testing"

    "github.com/jsamuelsen/go-service-template/internal/domain"
)

func BenchmarkQuoteValidation(b *testing.B) {
    quote := &domain.Quote{
        ID:      "quote-123",
        Content: "Test content",
        Author:  "Test Author",
    }

    for i := 0; i < b.N; i++ {
        _ = quote.Validate()
    }
}
```

### Benchmark with Setup

```go
func BenchmarkJSONMarshaling(b *testing.B) {
    // Setup (not measured)
    quote := &domain.Quote{
        ID:      "quote-123",
        Content: "This is a test quote with some content",
        Author:  "Test Author",
        Tags:    []string{"inspiration", "wisdom", "life"},
    }

    b.ResetTimer() // Don't measure setup

    for i := 0; i < b.N; i++ {
        _, _ = json.Marshal(quote)
    }
}
```

---

## Handler Benchmarks

**File:** `test/benchmark/handler_bench_test.go`

```go
package benchmark

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"

    "github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers"
    "github.com/jsamuelsen/go-service-template/internal/ports"
)

func init() {
    // Set Gin to release mode for accurate benchmarks
    gin.SetMode(gin.ReleaseMode)
}

// Helper: Create Gin context for testing
func createGinContext(w http.ResponseWriter, r *http.Request) *gin.Context {
    c, _ := gin.CreateTestContext(w)
    c.Request = r
    return c
}

// Helper: Setup handler with dependencies
func setupHealthHandler() *handlers.HealthHandler {
    registry := ports.NewHealthRegistry()
    buildInfo := handlers.NewBuildInfo("1.0.0", "abc123", "2024-01-01T00:00:00Z")
    return handlers.NewHealthHandler(registry, buildInfo)
}

// Benchmark liveness endpoint
func BenchmarkLivenessHandler(b *testing.B) {
    handler := setupHealthHandler()
    req := httptest.NewRequest(http.MethodGet, "/-/live", http.NoBody)

    b.ResetTimer()
    b.ReportAllocs() // Track memory allocations

    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        c := createGinContext(w, req)
        handler.Liveness(c)
    }
}

// Benchmark readiness with health checks
func BenchmarkReadinessHandler_WithChecks(b *testing.B) {
    registry := ports.NewHealthRegistry()

    // Add mock health checkers
    _ = registry.Register(&mockHealthChecker{name: "database"})
    _ = registry.Register(&mockHealthChecker{name: "cache"})

    buildInfo := handlers.NewBuildInfo("1.0.0", "abc123", "2024-01-01T00:00:00Z")
    handler := handlers.NewHealthHandler(registry, buildInfo)
    req := httptest.NewRequest(http.MethodGet, "/-/ready", http.NoBody)

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        c := createGinContext(w, req)
        handler.Readiness(c)
    }
}

// Mock health checker for benchmarks
type mockHealthChecker struct {
    name string
}

func (m *mockHealthChecker) Name() string {
    return m.name
}

func (m *mockHealthChecker) Check(_ context.Context) error {
    return nil
}
```

---

## Middleware Benchmarks

```go
func BenchmarkMiddlewareChain(b *testing.B) {
    router := gin.New()

    // Add middleware stack
    router.Use(gin.Recovery())

    router.GET("/test", func(c *gin.Context) {
        c.String(http.StatusOK, "ok")
    })

    req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
    }
}

func BenchmarkMiddlewareChain_Full(b *testing.B) {
    router := gin.New()

    // Full middleware stack
    router.Use(
        gin.Recovery(),
        gin.Logger(),
        // Add your custom middleware
    )

    router.GET("/test", func(c *gin.Context) {
        c.String(http.StatusOK, "ok")
    })

    req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
    }
}
```

---

## Sub-Benchmarks

Test variations with sub-benchmarks:

```go
func BenchmarkJSONParsing(b *testing.B) {
    sizes := []struct {
        name string
        size int
    }{
        {"small", 10},
        {"medium", 100},
        {"large", 1000},
    }

    for _, size := range sizes {
        b.Run(size.name, func(b *testing.B) {
            data := generateJSONPayload(size.size)

            b.ResetTimer()

            for i := 0; i < b.N; i++ {
                var result map[string]any
                _ = json.Unmarshal(data, &result)
            }
        })
    }
}
```

---

## Running Benchmarks

### Basic Run

```bash
# Run all benchmarks
task test:benchmark

# Or with go test
go test -bench=. ./test/benchmark/...
```

### With Memory Stats

```bash
# Include allocation stats
go test -bench=. -benchmem ./test/benchmark/...
```

### Specific Benchmark

```bash
# Run specific benchmark
go test -bench=BenchmarkLiveness ./test/benchmark/...

# Pattern matching
go test -bench=Handler ./test/benchmark/...
```

### Multiple Runs (for stability)

```bash
# Run 5 times for more stable results
go test -bench=. -count=5 ./test/benchmark/...
```

### Compare Results

```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Save baseline
go test -bench=. -count=5 ./test/benchmark/... > old.txt

# Make changes, then compare
go test -bench=. -count=5 ./test/benchmark/... > new.txt
benchstat old.txt new.txt
```

---

## Reading Results

```text
BenchmarkLivenessHandler-8    500000    2341 ns/op    1024 B/op    15 allocs/op
```

| Field                        | Meaning                       |
| ---------------------------- | ----------------------------- |
| `BenchmarkLivenessHandler-8` | Name and GOMAXPROCS           |
| `500000`                     | Iterations run                |
| `2341 ns/op`                 | Nanoseconds per operation     |
| `1024 B/op`                  | Bytes allocated per operation |
| `15 allocs/op`               | Allocations per operation     |

### Performance Targets

| Endpoint Type   | Target Latency | Target Allocs |
| --------------- | -------------- | ------------- |
| Health probes   | < 100 ns/op    | 0-2 allocs    |
| Simple GET      | < 10 µs/op     | < 20 allocs   |
| JSON processing | < 100 µs/op    | Varies        |

---

## Benchmark Best Practices

### 1. Avoid Compiler Optimizations

```go
// Bad: Compiler might optimize away
func BenchmarkBad(b *testing.B) {
    for i := 0; i < b.N; i++ {
        computeValue() // Result discarded
    }
}

// Good: Use result to prevent optimization
var result int

func BenchmarkGood(b *testing.B) {
    var r int
    for i := 0; i < b.N; i++ {
        r = computeValue()
    }
    result = r // Use result
}
```

### 2. Reset Timer After Setup

```go
func BenchmarkWithSetup(b *testing.B) {
    // Expensive setup
    data := loadLargeTestData()

    b.ResetTimer() // Don't measure setup

    for i := 0; i < b.N; i++ {
        process(data)
    }
}
```

### 3. Use b.ReportAllocs()

```go
func BenchmarkAllocations(b *testing.B) {
    b.ReportAllocs() // Track allocations

    for i := 0; i < b.N; i++ {
        _ = make([]byte, 1024)
    }
}
```

### 4. Test Realistic Scenarios

```go
// Good: Realistic data
func BenchmarkRealistic(b *testing.B) {
    quote := loadRealQuote() // Real-world data

    for i := 0; i < b.N; i++ {
        _ = processQuote(quote)
    }
}

// Bad: Trivial data
func BenchmarkTrivial(b *testing.B) {
    quote := &Quote{} // Empty/minimal data

    for i := 0; i < b.N; i++ {
        _ = processQuote(quote)
    }
}
```

---

## Checklist

- [ ] Function starts with `Benchmark`
- [ ] Uses `b.ResetTimer()` after setup
- [ ] Uses `b.ReportAllocs()` for memory tracking
- [ ] Loop runs `b.N` times
- [ ] Results aren't discarded (prevent compiler optimization)
- [ ] Tests realistic data sizes
- [ ] Located in `test/benchmark/`

---

## Related Documentation

- [Writing Unit Tests](./writing-unit-tests.md)
- [Writing Load Tests](./writing-load-tests.md)
