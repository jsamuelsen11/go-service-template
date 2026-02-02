# Playbook

Step-by-step how-to guides for common development tasks in this service template.

## Guides

### Adding Features

| Guide                                                         | Description                                                |
| ------------------------------------------------------------- | ---------------------------------------------------------- |
| [Adding an Endpoint](./adding-endpoint.md)                    | Create new HTTP endpoints with handlers, DTOs, and routing |
| [Adding a Downstream Client](./adding-downstream-client.md)   | Integrate with external services using the ACL pattern     |
| [Adding a Health Check](./adding-health-check.md)             | Implement health checks for dependencies                   |
| [Implementing a Feature Flag](./implementing-feature-flag.md) | Use feature flags for gradual rollouts                     |
| [Adding Custom Metrics](./adding-custom-metrics.md)           | Add OpenTelemetry metrics and Prometheus scraping          |

### Testing

| Guide                                                       | Description                                   |
| ----------------------------------------------------------- | --------------------------------------------- |
| [Writing Unit Tests](./writing-unit-tests.md)               | Table-driven tests, mocking, and test helpers |
| [Writing Integration Tests](./writing-integration-tests.md) | GoDog/Cucumber BDD tests with Gherkin         |
| [Writing Benchmark Tests](./writing-benchmark-tests.md)     | Go benchmarks for performance testing         |
| [Writing Load Tests](./writing-load-tests.md)               | k6 scripts for load and stress testing        |

## Quick Reference

### Task Commands

```bash
task run               # Run service with hot reload
task test              # Run unit tests
task test:integration  # Run BDD integration tests
task test:benchmark    # Run Go benchmarks
task test:load         # Run k6 load tests
task generate          # Regenerate mocks
task lint              # Run linter
```

### Key Directories

| Directory                    | Purpose                                             |
| ---------------------------- | --------------------------------------------------- |
| `internal/domain/`           | Business entities and domain errors                 |
| `internal/ports/`            | Interface definitions (contracts)                   |
| `internal/app/`              | Application services (use case orchestration)       |
| `internal/adapters/http/`    | HTTP handlers, middleware, DTOs                     |
| `internal/adapters/clients/` | Downstream HTTP clients                             |
| `internal/platform/`         | Cross-cutting concerns (config, logging, telemetry) |
| `test/integration/`          | GoDog integration tests                             |
| `test/benchmark/`            | Go benchmark tests                                  |
| `test/load/k6/`              | k6 load test scripts                                |

## Related Documentation

- [Architecture](../ARCHITECTURE.md) - System design and layer descriptions
- [Patterns](../PATTERNS.md) - Go patterns for concurrency and error handling
- [Secret Redaction](../SECRET_REDACTION.md) - Logging security guidelines
