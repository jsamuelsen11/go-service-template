# Troubleshooting

This guide helps diagnose and resolve common issues with the Go Service Template.

## Table of Contents

- [Troubleshooting](#troubleshooting)
  - [Table of Contents](#table-of-contents)
  - [Quick Diagnostics](#quick-diagnostics)
  - [Service Won't Start](#service-wont-start)
    - [Configuration Validation Errors](#configuration-validation-errors)
    - [Port Already in Use](#port-already-in-use)
    - [Missing Configuration Files](#missing-configuration-files)
  - [Health Checks Failing](#health-checks-failing)
    - [Readiness Check Returns 503](#readiness-check-returns-503)
    - [Individual Component Failing](#individual-component-failing)
  - [Tracing Not Working](#tracing-not-working)
    - [No Traces Appearing](#no-traces-appearing)
    - [Trace Context Not Propagating](#trace-context-not-propagating)
  - [High Latency Issues](#high-latency-issues)
    - [Slow Downstream Calls](#slow-downstream-calls)
    - [Circuit Breaker Tripping](#circuit-breaker-tripping)
  - [Memory Issues](#memory-issues)
    - [High Memory Usage](#high-memory-usage)
    - [Memory Leaks](#memory-leaks)
  - [Connection Pool Exhaustion](#connection-pool-exhaustion)
    - [Too Many Open Connections](#too-many-open-connections)
    - [Connection Timeouts](#connection-timeouts)
  - [Build Problems](#build-problems)
    - [Go Module Issues](#go-module-issues)
    - [Build Failures](#build-failures)
    - [CGO Issues](#cgo-issues)
  - [Test Failures](#test-failures)
    - [Unit Test Failures](#unit-test-failures)
    - [Integration Test Failures](#integration-test-failures)
    - [Benchmark Test Issues](#benchmark-test-issues)
  - [Tool Installation Issues](#tool-installation-issues)
    - [Go Tool Not Found](#go-tool-not-found)
    - [Mise and Go Version Issues](#mise-and-go-version-issues)
    - [Lefthook Not Working](#lefthook-not-working)
    - [k6 Not Installed](#k6-not-installed)
    - [Docker Issues](#docker-issues)

---

## Quick Diagnostics

Run these commands first to gather system state:

```bash
# Check if service is running
curl -s http://localhost:8080/-/live | jq .

# Check readiness and component health
curl -s http://localhost:8080/-/ready | jq .

# Check build info
curl -s http://localhost:8080/-/build | jq .

# View Prometheus metrics
curl -s http://localhost:8080/-/metrics | grep -E "^(http_|go_)"

# Check service logs (if running via task)
task run 2>&1 | head -100
```

---

## Service Won't Start

### Configuration Validation Errors

**Symptoms:**

| Symptom                              | Example                                                                     |
| ------------------------------------ | --------------------------------------------------------------------------- |
| Service exits immediately on startup | `exit code 1`                                                               |
| Validation error in logs             | `validation error: app.environment must be one of [local dev qa prod test]` |
| Unmarshal error                      | `unmarshalling config: ...`                                                 |

**Causes:**

| Cause                  | Description                                                      |
| ---------------------- | ---------------------------------------------------------------- |
| Invalid config value   | Value doesn't match validation rules (e.g., invalid environment) |
| Type mismatch          | Wrong type in YAML (e.g., string instead of duration)            |
| Missing required field | Required field not set and no default                            |

**Solutions:**

1. **Check config file syntax:**

   ```bash
   # Validate YAML syntax
   cat configs/local.yaml | python3 -c "import sys, yaml; yaml.safe_load(sys.stdin)"
   ```

2. **Review valid configuration values:**

   | Field             | Valid Values                         |
   | ----------------- | ------------------------------------ |
   | `app.environment` | `local`, `dev`, `qa`, `prod`, `test` |
   | `log.level`       | `debug`, `info`, `warn`, `error`     |
   | `log.format`      | `json`, `text`                       |
   | `server.port`     | `1` - `65535`                        |

3. **Check duration format:**

   ```yaml
   # Correct duration formats
   server:
     read_timeout: "30s" # seconds
     write_timeout: "1m" # minutes
     idle_timeout: "2m30s" # combined
   ```

4. **Verify environment variable overrides:**

   ```bash
   # List all APP_ environment variables
   env | grep "^APP_" | sort

   # Environment variables use underscore separator
   # APP_SERVER_PORT=9090 sets server.port
   ```

**Debug Commands:**

```bash
# Load config and print parsed values (add to main.go temporarily)
go run ./cmd/service -profile=local 2>&1

# Check for config file existence
ls -la configs/

# Validate specific profile
APP_PROFILE=local go run ./cmd/service
```

---

### Port Already in Use

**Symptoms:**

| Symptom                     | Example                                          |
| --------------------------- | ------------------------------------------------ |
| Bind error on startup       | `listen tcp :8080: bind: address already in use` |
| Service crashes immediately | Exit with network error                          |

**Causes:**

- Another instance of the service is running
- Different service using the same port
- Zombie process holding the port

**Solutions:**

```bash
# Find process using port 8080
lsof -i :8080

# Or using ss
ss -tlnp | grep 8080

# Kill the process if needed
kill -9 <PID>

# Or use a different port via environment variable
APP_SERVER_PORT=9090 task run
```

---

### Missing Configuration Files

**Symptoms:**

| Symptom                      | Example                              |
| ---------------------------- | ------------------------------------ |
| Service starts with defaults | Unexpected default values            |
| Warning about missing file   | (Silent - missing files are ignored) |

**Causes:**

- Config file doesn't exist at expected path
- Wrong working directory
- Profile name mismatch

**Solutions:**

```bash
# Check current working directory
pwd

# List available config files
ls -la configs/

# Run with explicit profile
APP_PROFILE=local task run

# Check config load order (highest to lowest precedence):
# 1. Environment variables (APP_ prefix)
# 2. Profile config (configs/{profile}.yaml)
# 3. Base config (configs/base.yaml)
# 4. Default values
```

---

## Health Checks Failing

### Readiness Check Returns 503

**Symptoms:**

| Symptom                             | Example                                             |
| ----------------------------------- | --------------------------------------------------- |
| 503 Service Unavailable             | `curl -i http://localhost:8080/-/ready` returns 503 |
| Status shows "unhealthy"            | `{"status":"unhealthy","checks":{...}}`             |
| Kubernetes removes pod from service | Pod not receiving traffic                           |

**Causes:**

| Cause                          | Description                    |
| ------------------------------ | ------------------------------ |
| Downstream service unavailable | External API not reachable     |
| Database connection failed     | DB health check failing        |
| Circuit breaker open           | Downstream marked as unhealthy |

**Solutions:**

1. **Identify failing component:**

   ```bash
   # Get detailed health status
   curl -s http://localhost:8080/-/ready | jq '.checks'

   # Example output:
   # {
   #   "quote-service": {
   #     "status": "unhealthy",
   #     "message": "connection refused",
   #     "duration": 5000000000
   #   }
   # }
   ```

2. **Check specific downstream:**

   ```bash
   # Test downstream directly
   curl -v https://api.quotable.io/random

   # Check DNS resolution
   nslookup api.quotable.io

   # Test network connectivity
   nc -zv api.quotable.io 443
   ```

3. **Review circuit breaker state in logs:**

   ```bash
   # Look for circuit breaker state changes
   task run 2>&1 | grep "circuit breaker"
   ```

---

### Individual Component Failing

**Symptoms:**

- One specific check shows `"status": "unhealthy"`
- Other checks pass normally

**Solutions:**

| Component Type | Diagnostic Steps                                      |
| -------------- | ----------------------------------------------------- |
| Database       | Check connection string, credentials, network access  |
| External API   | Verify URL, check firewall rules, test authentication |

```bash
# Check component-specific logs
task run 2>&1 | grep -E "(database|redis|api)"

# Test network to component
ping <component-host>
telnet <component-host> <port>
```

---

## Tracing Not Working

### No Traces Appearing

**Symptoms:**

| Symptom                     | Example                        |
| --------------------------- | ------------------------------ |
| No spans in tracing backend | Jaeger/Zipkin shows no data    |
| No trace IDs in logs        | Missing `trace_id` field       |
| Telemetry provider errors   | `creating trace exporter: ...` |

**Causes:**

| Cause                 | Description                    |
| --------------------- | ------------------------------ |
| Telemetry disabled    | `telemetry.enabled: false`     |
| Wrong endpoint        | Collector endpoint unreachable |
| Sampling rate zero    | `telemetry.sampling_rate: 0`   |
| Collector not running | OTLP collector not started     |

**Solutions:**

1. **Verify telemetry is enabled:**

   ```yaml
   # configs/local.yaml
   telemetry:
     enabled: true
     endpoint: "localhost:4317" # OTLP gRPC endpoint
     service_name: "go-service-template"
     sampling_rate: 1.0 # 1.0 = 100% sampling
   ```

2. **Check OTLP collector is running:**

   ```bash
   # Check if collector is listening
   nc -zv localhost 4317

   # If using Docker
   docker ps | grep otel-collector

   # Start Jaeger with OTLP support
   docker run -d --name jaeger \
     -p 16686:16686 \
     -p 4317:4317 \
     jaegertracing/all-in-one:latest
   ```

3. **Verify endpoint format:**

   ```yaml
   # Correct: host:port without scheme for gRPC
   telemetry:
     endpoint: "localhost:4317"

   # Wrong: don't include http:// or grpc://
   # endpoint: "http://localhost:4317"  # incorrect
   ```

4. **Check for startup errors:**

   ```bash
   task run 2>&1 | grep -i "telemetry\|trace\|otel"
   ```

---

### Trace Context Not Propagating

**Symptoms:**

| Symptom                          | Example                                  |
| -------------------------------- | ---------------------------------------- |
| Separate traces for each service | No parent-child relationship             |
| Missing `traceparent` header     | Downstream doesn't receive trace context |

**Causes:**

- Not using instrumented HTTP client
- Headers stripped by proxy
- Downstream doesn't support W3C Trace Context

**Solutions:**

1. **Verify using instrumented client:**

   ```go
   // Correct: use clients.Client
   client, _ := clients.New(&clients.Config{...})
   resp, err := client.Get(ctx, "/path")

   // Wrong: standard http.Client won't propagate traces
   // resp, err := http.Get(url)
   ```

2. **Check propagated headers:**

   ```bash
   # Inspect outgoing headers (add debug logging)
   curl -v http://localhost:8080/api/endpoint 2>&1 | grep -i trace
   ```

3. **Verify propagator configuration:**
   The service uses W3C Trace Context by default. Check that downstream services also support it.

---

## High Latency Issues

### Slow Downstream Calls

**Symptoms:**

| Symptom                | Example                       |
| ---------------------- | ----------------------------- |
| High p99 latency       | Requests taking >1s           |
| Timeout errors         | `context deadline exceeded`   |
| Retry warnings in logs | `retrying request, attempt=2` |

**Causes:**

| Cause              | Description                          |
| ------------------ | ------------------------------------ |
| Downstream is slow | External service responding slowly   |
| Network issues     | High latency to downstream           |
| Retry storms       | Multiple retries compounding latency |
| Timeout too short  | Request can't complete in time       |

**Solutions:**

1. **Check downstream latency:**

   ```bash
   # Measure direct call to downstream
   time curl -s https://api.quotable.io/random > /dev/null

   # Check via service
   time curl -s http://localhost:8080/api/quote > /dev/null
   ```

2. **Review Prometheus metrics:**

   ```bash
   # Get client request duration histogram
   curl -s http://localhost:8080/-/metrics | grep "http_client_request_duration"
   ```

3. **Adjust timeout configuration:**

   ```yaml
   client:
     timeout: "30s" # Per-attempt timeout
     retry:
       max_attempts: 3
       initial_interval: "100ms"
       max_interval: "5s"
       multiplier: 2.0
   ```

4. **Check retry behavior:**

   ```bash
   # Watch for retry logs
   task run 2>&1 | grep -E "retry|attempt"
   ```

---

### Circuit Breaker Tripping

**Symptoms:**

| Symptom                         | Example                                             |
| ------------------------------- | --------------------------------------------------- |
| Requests immediately fail       | `circuit breaker is open` error                     |
| Warning logs about state change | `circuit breaker state changed from=closed to=open` |
| 503 errors to clients           | Service unavailable responses                       |

**Causes:**

- Downstream experiencing failures
- Too aggressive circuit breaker settings
- Network instability

**Solutions:**

1. **Check circuit breaker state:**

   ```bash
   # Look for state transitions in logs
   task run 2>&1 | grep "circuit breaker state"
   ```

2. **Review circuit breaker configuration:**

   ```yaml
   client:
     circuit_breaker:
       max_failures: 5 # Failures before opening
       timeout: "30s" # Time before trying half-open
       half_open_limit: 3 # Successes to close again
   ```

3. **Understand state transitions:**

   | Transition          | Trigger                             |
   | ------------------- | ----------------------------------- |
   | Closed -> Open      | `max_failures` consecutive failures |
   | Open -> Half-Open   | `timeout` duration passes           |
   | Half-Open -> Closed | `half_open_limit` successes         |
   | Half-Open -> Open   | Any failure                         |

4. **Wait for recovery:**
   The circuit will automatically try to recover after the timeout. Monitor logs for `half-open` state.

---

## Memory Issues

### High Memory Usage

**Symptoms:**

| Symptom                 | Example                                     |
| ----------------------- | ------------------------------------------- |
| OOM kills               | Container restarts due to memory limit      |
| High RSS in metrics     | `go_memstats_alloc_bytes` consistently high |
| Slow garbage collection | Long GC pauses                              |

**Causes:**

| Cause                 | Description                   |
| --------------------- | ----------------------------- |
| Large response bodies | Not closing response bodies   |
| Goroutine leaks       | Goroutines not terminating    |
| Large request bodies  | Processing oversized requests |

**Solutions:**

1. **Check memory metrics:**

   ```bash
   # Get Go memory stats
   curl -s http://localhost:8080/-/metrics | grep go_memstats

   # Key metrics to watch:
   # go_memstats_alloc_bytes - current allocated memory
   # go_memstats_heap_inuse_bytes - heap memory in use
   # go_goroutines - number of goroutines
   ```

2. **Profile memory usage:**

   ```bash
   # If pprof is enabled
   go tool pprof http://localhost:8080/debug/pprof/heap

   # Get memory profile
   curl -s http://localhost:8080/debug/pprof/heap > heap.prof
   go tool pprof -top heap.prof
   ```

3. **Check for goroutine leaks:**

   ```bash
   # Get goroutine count
   curl -s http://localhost:8080/-/metrics | grep go_goroutines

   # Should be relatively stable under load
   # Growing count indicates leak
   ```

4. **Review max request size:**

   ```yaml
   server:
     max_request_size: 1048576 # 1MB default
   ```

---

### Memory Leaks

**Symptoms:**

| Symptom                   | Example                             |
| ------------------------- | ----------------------------------- |
| Memory grows over time    | Continuous increase without plateau |
| Goroutine count increases | `go_goroutines` metric growing      |

**Solutions:**

1. **Common leak patterns to check:**

   | Pattern                  | Fix                                |
   | ------------------------ | ---------------------------------- |
   | Response body not closed | Always `defer resp.Body.Close()`   |
   | Context not cancelled    | Use `defer cancel()` with timeouts |
   | Channel not closed       | Producer must close channels       |
   | Ticker not stopped       | Use `defer ticker.Stop()`          |

2. **Profile goroutines:**

   ```bash
   # Get goroutine stack traces
   curl -s http://localhost:8080/debug/pprof/goroutine?debug=2 > goroutines.txt

   # Look for stuck goroutines
   grep -A 5 "goroutine" goroutines.txt | head -50
   ```

---

## Connection Pool Exhaustion

### Too Many Open Connections

**Symptoms:**

| Symptom                       | Example                                |
| ----------------------------- | -------------------------------------- |
| Connection refused errors     | `dial tcp: too many open files`        |
| Socket exhaustion             | System-level resource limits hit       |
| Slow connection establishment | Long time to establish new connections |

**Causes:**

| Cause                      | Description                  |
| -------------------------- | ---------------------------- |
| Response bodies not closed | Connections stay open        |
| High concurrency           | More requests than pool size |
| Slow downstream            | Connections blocked waiting  |

**Solutions:**

1. **Check system limits:**

   ```bash
   # Check open file limit
   ulimit -n

   # Check current open connections
   lsof -p $(pgrep -f "go-service") | wc -l

   # Check connections to specific host
   ss -tn | grep <downstream-host> | wc -l
   ```

2. **Review HTTP client pool settings:**

   ```go
   // Default pool settings in client.go
   Transport: &http.Transport{
       MaxIdleConns:        100,  // Total idle connections
       MaxIdleConnsPerHost: 10,   // Per-host idle connections
       IdleConnTimeout:     90 * time.Second,
   }
   ```

3. **Ensure response bodies are closed:**

   ```go
   // Correct pattern
   resp, err := client.Get(ctx, "/path")
   if err != nil {
       return err
   }
   defer resp.Body.Close()  // Always close!

   // Read body
   body, err := io.ReadAll(resp.Body)
   ```

---

### Connection Timeouts

**Symptoms:**

| Symptom                     | Example                    |
| --------------------------- | -------------------------- |
| `i/o timeout` errors        | Network-level timeout      |
| `context deadline exceeded` | Context timeout            |
| Requests stuck waiting      | No response within timeout |

**Causes:**

- Downstream not responding
- Network issues
- DNS resolution slow
- Timeout configured too low

**Solutions:**

1. **Test network connectivity:**

   ```bash
   # Check DNS resolution
   time nslookup <downstream-host>

   # Test TCP connectivity
   nc -zv <downstream-host> <port>

   # Measure latency
   ping -c 5 <downstream-host>
   ```

2. **Review timeout configuration:**

   ```yaml
   client:
     timeout: "30s" # Per-request timeout

   server:
     read_timeout: "30s"
     write_timeout: "30s"
     idle_timeout: "120s"
   ```

3. **Check for DNS caching issues:**

   ```bash
   # Clear DNS cache (Linux)
   sudo systemd-resolve --flush-caches

   # Check current resolution
   dig <downstream-host>
   ```

4. **Monitor connection reuse:**

   ```bash
   # Look for "Connection: keep-alive" in responses
   curl -v http://localhost:8080/api/endpoint 2>&1 | grep -i connection
   ```

---

## Build Problems

### Go Module Issues

**Symptoms:**

| Symptom              | Example                                        |
| -------------------- | ---------------------------------------------- |
| Missing dependencies | `cannot find package "github.com/..."`         |
| Version conflicts    | `ambiguous import: found package in two paths` |
| Checksum mismatch    | `verifying: checksum mismatch`                 |
| Module not found     | `module not found`                             |

**Causes:**

| Cause                 | Description                           |
| --------------------- | ------------------------------------- |
| Stale module cache    | Cached modules don't match go.sum     |
| Corrupted go.sum      | Checksum file out of sync             |
| Private module access | Missing credentials for private repos |
| Proxy issues          | GOPROXY not reachable                 |

**Solutions:**

1. **Clean module cache and re-download:**

   ```bash
   go clean -modcache
   go mod download
   ```

2. **Tidy modules:**

   ```bash
   go mod tidy
   ```

3. **Verify checksums:**

   ```bash
   go mod verify
   ```

4. **Update dependencies:**

   ```bash
   go get -u ./...
   go mod tidy
   ```

5. **Check proxy settings:**

   ```bash
   # View current proxy
   go env GOPROXY

   # Use direct access if proxy has issues
   GOPROXY=direct go mod download
   ```

---

### Build Failures

**Symptoms:**

| Symptom           | Example                           |
| ----------------- | --------------------------------- |
| Compilation error | `undefined: SomeFunction`         |
| Type mismatch     | `cannot use X (type A) as type B` |
| Import cycle      | `import cycle not allowed`        |
| Missing generated | `undefined: MockSomeInterface`    |

**Causes:**

| Cause                  | Description                      |
| ---------------------- | -------------------------------- |
| Missing generated code | Mocks or generated files missing |
| Wrong Go version       | Syntax not supported             |
| Stale build cache      | Cached artifacts out of date     |
| Missing dependencies   | go.mod not updated               |

**Solutions:**

1. **Regenerate mocks and generated code:**

   ```bash
   task generate
   git status  # Check for new/modified generated files
   ```

2. **Verify Go version matches go.mod:**

   ```bash
   go version
   # Should match: go 1.25.7

   # If wrong version, use mise to install correct version
   mise use go@1.25.7
   ```

3. **Clean build cache:**

   ```bash
   go clean -cache
   task build
   ```

4. **Re-download dependencies:**

   ```bash
   go mod download
   ```

---

### CGO Issues

**Symptoms:**

| Symptom                 | Example                             |
| ----------------------- | ----------------------------------- |
| CGO disabled errors     | `cgo: C compiler not found`         |
| Missing C compiler      | `gcc: command not found`            |
| Cross-compilation fails | Architecture-specific linker errors |

**Causes:**

- CGO required but C compiler not installed
- Cross-compiling without disabling CGO
- Missing system libraries

**Solutions:**

1. **Disable CGO (if not needed):**

   ```bash
   CGO_ENABLED=0 task build
   CGO_ENABLED=0 go build ./...
   ```

2. **Install C compiler (if CGO needed):**

   ```bash
   # macOS
   xcode-select --install

   # Ubuntu/Debian
   sudo apt-get install build-essential

   # Alpine
   apk add gcc musl-dev

   # Fedora/RHEL
   sudo dnf install gcc
   ```

3. **Cross-compilation without CGO:**

   ```bash
   # Build for Linux from macOS
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...
   ```

---

## Test Failures

### Unit Test Failures

**Symptoms:**

| Symptom              | Example                           |
| -------------------- | --------------------------------- |
| Test assertion fails | `expected X, got Y`               |
| Race condition       | `WARNING: DATA RACE`              |
| Timeout              | `panic: test timed out after 30s` |
| Missing mocks        | `undefined: MockSomeInterface`    |

**Causes:**

| Cause          | Description                       |
| -------------- | --------------------------------- |
| Code bug       | Logic error in implementation     |
| Race condition | Concurrent access to shared state |
| Slow test      | Test takes too long               |
| Outdated mocks | Mocks don't match interface       |

**Solutions:**

1. **Run specific test with verbose output:**

   ```bash
   go test -v -run TestSpecificName ./path/to/package/...
   ```

2. **Run with race detector (default in task test):**

   ```bash
   go test -race ./...
   ```

3. **Increase timeout:**

   ```bash
   go test -timeout 5m ./...
   ```

4. **Regenerate mocks:**

   ```bash
   task generate
   ```

5. **Debug a specific test:**

   ```bash
   # With verbose output
   go test -v -run TestName ./pkg/...

   # With debug logging
   go test -v -run TestName ./pkg/... 2>&1 | less
   ```

---

### Integration Test Failures

**Symptoms:**

| Symptom                | Example                                                   |
| ---------------------- | --------------------------------------------------------- |
| Connection refused     | `dial tcp 127.0.0.1:8080: connect: connection refused`    |
| Service not ready      | `Get "http://localhost:8080/-/ready": connection refused` |
| Test data not found    | Gherkin step not matching                                 |
| Feature file not found | `no feature file provided`                                |

**Causes:**

| Cause               | Description                              |
| ------------------- | ---------------------------------------- |
| Service not running | Integration tests require running server |
| Wrong port          | Service running on different port        |
| Missing test data   | Gherkin steps not implemented            |

**Solutions:**

1. **Ensure service is running first:**

   ```bash
   # Terminal 1: Start service
   task run

   # Terminal 2: Run integration tests
   task test:integration
   ```

2. **Check service health:**

   ```bash
   curl http://localhost:8080/-/live
   ```

3. **Run specific scenario by tag:**

   ```bash
   GODOG_TAGS="@specific-tag" task test:integration
   ```

4. **Run all integration tests (not just smoke):**

   ```bash
   task test:integration:all
   ```

5. **Check test configuration:**

   ```bash
   # Override base URL if service is on different port
   BASE_URL=http://localhost:9090 task test:integration
   ```

---

### Benchmark Test Issues

**Symptoms:**

| Symptom              | Example                             |
| -------------------- | ----------------------------------- |
| No benchmarks run    | `testing: warning: no tests to run` |
| Inconsistent results | Wildly varying benchmark times      |
| Benchmark not found  | `no matching benchmarks`            |

**Causes:**

| Cause               | Description                       |
| ------------------- | --------------------------------- |
| Wrong file location | Benchmark not in test/benchmark/  |
| Wrong function name | Not prefixed with `Benchmark`     |
| System noise        | Other processes affecting results |

**Solutions:**

1. **Verify benchmark file location:**

   ```bash
   ls test/benchmark/
   ```

2. **Run specific benchmark:**

   ```bash
   go test -bench=BenchmarkSpecific -benchmem ./test/benchmark/...
   ```

3. **For consistent results, run multiple times:**

   ```bash
   go test -bench=. -benchmem -count=5 ./test/benchmark/...
   ```

4. **Benchmark function naming:**

   ```go
   // Correct: starts with "Benchmark"
   func BenchmarkHandler(b *testing.B) { ... }

   // Wrong: won't be recognized
   func TestBenchmarkHandler(b *testing.B) { ... }
   ```

---

## Tool Installation Issues

### Go Tool Not Found

**Symptoms:**

| Symptom             | Example                            |
| ------------------- | ---------------------------------- |
| Tool not found      | `go tool golangci-lint: not found` |
| Command not in PATH | `command not found: golangci-lint` |

**Causes:**

| Cause               | Description               |
| ------------------- | ------------------------- |
| Setup not run       | `task setup` not executed |
| Tools not in go.mod | Tool directives missing   |
| Go bin not in PATH  | GOBIN not in PATH         |

**Solutions:**

1. **Re-run setup:**

   ```bash
   task setup
   ```

2. **Verify tools are in go.mod:**

   ```bash
   grep "tool" go.mod
   ```

3. **Download tools:**

   ```bash
   go mod download
   ```

4. **Run tool directly:**

   ```bash
   # Using go tool (preferred)
   go tool golangci-lint run ./...

   # Or install globally
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

---

### Mise and Go Version Issues

**Symptoms:**

| Symptom          | Example                             |
| ---------------- | ----------------------------------- |
| Wrong Go version | Build errors, syntax not recognized |
| mise not found   | `mise: command not found`           |
| Version mismatch | `go.mod requires go 1.25.7`         |

**Causes:**

| Cause                   | Description                   |
| ----------------------- | ----------------------------- |
| mise not installed      | Version manager not set up    |
| Wrong Go version active | Different version in PATH     |
| Shell not configured    | mise not initialized in shell |

**Solutions:**

1. **Install mise:**

   ```bash
   curl https://mise.run | sh
   ```

2. **Initialize in shell (add to ~/.bashrc or ~/.zshrc):**

   ```bash
   eval "$(mise activate bash)"  # or zsh
   ```

3. **Install correct Go version:**

   ```bash
   mise use go@1.25.7
   ```

4. **Verify version:**

   ```bash
   go version
   # Expected: go version go1.25.7 ...
   ```

5. **Check mise status:**

   ```bash
   mise doctor
   mise list
   ```

---

### Lefthook Not Working

**Symptoms:**

| Symptom           | Example                        |
| ----------------- | ------------------------------ |
| Hooks not running | Commits succeed without checks |
| Hook errors       | `lefthook: command not found`  |
| Hooks outdated    | New hooks not executing        |

**Causes:**

| Cause                | Description               |
| -------------------- | ------------------------- |
| Hooks not installed  | `task setup` not run      |
| Hooks outdated       | lefthook.yml changed      |
| Hook scripts missing | .git/hooks/ files deleted |

**Solutions:**

1. **Reinstall hooks:**

   ```bash
   go tool lefthook install
   ```

2. **Verify hooks are installed:**

   ```bash
   ls -la .git/hooks/
   # Should show: pre-commit, commit-msg, pre-push
   ```

3. **Run hooks manually to debug:**

   ```bash
   go tool lefthook run pre-commit
   ```

4. **Enable verbose mode:**

   ```bash
   LEFTHOOK_VERBOSE=1 git commit -m "test"
   ```

5. **Check lefthook checksum:**

   ```bash
   cat .git/info/lefthook.checksum
   ```

---

### k6 Not Installed

**Symptoms:**

| Symptom         | Example                                |
| --------------- | -------------------------------------- |
| Load tests fail | `k6: command not found`                |
| Task errors     | `task: Failed to run task "test:load"` |

**Causes:**

- k6 not installed on system
- k6 not in PATH

**Solutions:**

1. **Install via task:**

   ```bash
   task setup:k6
   ```

2. **Manual installation:**

   ```bash
   # macOS
   brew install k6

   # Ubuntu/Debian
   sudo gpg -k
   sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
     --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
   echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" \
     | sudo tee /etc/apt/sources.list.d/k6.list
   sudo apt-get update
   sudo apt-get install k6

   # Windows
   choco install k6
   ```

3. **Verify installation:**

   ```bash
   k6 version
   ```

---

### Docker Issues

**Symptoms:**

| Symptom             | Example                                     |
| ------------------- | ------------------------------------------- |
| hadolint hook fails | `Cannot connect to the Docker daemon`       |
| docker:build fails  | `docker: command not found`                 |
| Permission denied   | `permission denied while trying to connect` |

**Causes:**

| Cause                | Description                  |
| -------------------- | ---------------------------- |
| Docker not running   | Docker daemon not started    |
| Docker not installed | Docker Desktop not installed |
| Permission issues    | User not in docker group     |

**Solutions:**

1. **Start Docker daemon:**

   ```bash
   # macOS/Windows: Start Docker Desktop from applications

   # Linux
   sudo systemctl start docker
   ```

2. **Verify Docker is running:**

   ```bash
   docker ps
   ```

3. **Add user to docker group (Linux):**

   ```bash
   sudo usermod -aG docker $USER
   # Log out and back in for changes to take effect
   ```

4. **Skip hadolint temporarily (if Docker unavailable):**

   ```bash
   LEFTHOOK=0 git commit -m "message"
   ```

5. **Install Docker:**

   ```bash
   # macOS
   brew install --cask docker

   # Ubuntu
   sudo apt-get install docker.io

   # Or download Docker Desktop from https://www.docker.com/products/docker-desktop
   ```
