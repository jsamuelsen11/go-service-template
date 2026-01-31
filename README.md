# Go Service Template

Enterprise-grade Go backend service template implementing Clean/Hexagonal Architecture.

## Architecture

<!-- TODO: Add architecture diagram after implementation -->

## Installation

<!-- TODO: Add installation instructions after implementation -->

## Run Locally

<!-- TODO: Add run instructions after implementation -->

## Tests

<!-- TODO: Add test instructions after implementation -->

## Task Commands

```bash
task setup             # Install all development dependencies
task run               # Run the service locally with hot reload
task build             # Build production binary with version info
task fmt               # Format all Go code
task lint              # Run golangci-lint
task test              # Run unit tests
task test:integration  # Run GoDog integration tests
task test:e2e          # Run full E2E test suite
task test:benchmark    # Run Go benchmarks
task test:load         # Run k6 load tests
task coverage          # Generate test coverage report
task generate          # Run go generate (mocks, etc.)
task ci                # Run full CI pipeline locally
task docker:build      # Build Docker image
task docker:run        # Run Docker container locally
task openapi:validate  # Validate OpenAPI spec
task openapi:diff      # Check for OpenAPI drift
task vuln              # Run govulncheck
```
