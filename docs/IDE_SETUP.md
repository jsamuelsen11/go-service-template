# IDE Setup Guide

This document maps the Go tools used in this project to their IDE integrations.

## Go Tools → IDE Extensions

| Go Tool           | Purpose                        | VS Code Extension                  | GoLand             |
| ----------------- | ------------------------------ | ---------------------------------- | ------------------ |
| **golangci-lint** | Linting (22+ linters)          | `golang.go` (built-in)             | Built-in           |
| **gofumpt**       | Formatting (stricter gofmt)    | `golang.go` (via gopls)            | File Watcher       |
| **gotestsum**     | Test runner with better output | `golang.go` (Test Explorer)        | N/A (use CLI)      |
| **govulncheck**   | Vulnerability scanning         | N/A (use CLI)                      | Built-in (2023.3+) |
| **mockery**       | Mock generation                | N/A (use CLI)                      | N/A (use CLI)      |
| **air**           | Hot reload                     | N/A (use terminal)                 | N/A (use terminal) |
| **godog**         | BDD/Cucumber testing           | `alexkrechik.cucumberautocomplete` | Cucumber plugin    |
| **vacuum**        | OpenAPI linting                | `stoplight.spectral`               | N/A                |
| **oasdiff**       | OpenAPI diff                   | N/A (use CLI)                      | N/A (use CLI)      |
| **lefthook**      | Git hooks                      | N/A (runs automatically)           | N/A                |
| **gitleaks**      | Secret detection               | N/A (runs via hooks)               | N/A                |
| **actionlint**    | GitHub Actions linting         | N/A (runs via hooks)               | N/A                |

## VS Code Setup

### Required Extensions

Install the recommended extensions when prompted, or run:

```bash
code --install-extension golang.go
```

The `.vscode/extensions.json` file contains all recommendations.

### Key Settings

The `.vscode/settings.json` configures:

- **Linting**: golangci-lint runs on save
- **Formatting**: gofumpt via gopls on save
- **Testing**: Race detection enabled, coverage on test
- **gopls**: Semantic tokens, code lenses enabled

### Debug Configurations

Available in `.vscode/launch.json`:

1. **Launch Service** - Run the service with debugger
2. **Launch Service (Debug Logging)** - With debug log level
3. **Debug Package Tests** - Test current package
4. **Debug Test at Cursor** - Run selected test
5. **Debug Integration Tests** - BDD tests
6. **Debug Benchmarks** - Benchmark tests
7. **Attach to Process** - Attach to running service

## GoLand Setup

### Linter Configuration

1. Go to **Settings → Tools → Go Linter**
2. Select **golangci-lint**
3. Point to project's `.golangci.yml`

### Formatter Configuration

1. Go to **Settings → Tools → File Watchers**
2. Add watcher for `gofumpt`:
   - File type: Go
   - Program: `go`
   - Arguments: `tool gofumpt -w $FilePath$`

### Run Configurations

Import from `.idea/runConfigurations/`:

- `Run_Service.xml` - Basic service launch
- `Run_Service__Debug_Logging_.xml` - With debug logging
- Task runner integrations for common tasks

## Tool Details

### golangci-lint

Runs 22+ linters including:

- `govet`, `errcheck`, `staticcheck` (bugs)
- `gosec` (security)
- `gocritic`, `revive` (style)
- `gocyclo`, `gocognit` (complexity)

Config: `.golangci.yml`

### gotestsum

Better test output formatting. The VS Code Go extension's Test Explorer uses `go test`
under the hood. Use `gotestsum` via CLI for nicer output.

```bash
task test                    # Uses gotestsum
task test TEST_FORMAT=dots   # Minimal output
```

### govulncheck

Checks dependencies for known vulnerabilities:

```bash
task vuln
```

GoLand 2023.3+ has built-in integration. VS Code users run via CLI.

### mockery

Generates mocks from interfaces. Config in `.mockery.yaml`:

```bash
task generate  # Runs mockery
```

### air

Hot reload for development. Config in `.air.toml`:

```bash
task run  # Uses air for hot reload
```

## Recommended Workflow

1. **Open project** - VS Code will prompt to install recommended extensions
2. **Run `task setup`** - Installs dependencies and git hooks
3. **Use `task run`** - Starts service with hot reload
4. **Save files** - Auto-formats and lints on save
5. **Run tests** - Use Test Explorer or `task test`

## Troubleshooting

### Linting not working

1. Ensure golangci-lint is available: `go tool golangci-lint --version`
2. Check VS Code output panel for errors
3. Verify `.golangci.yml` is valid

### Formatting not working

1. Check gopls is running (Go extension status)
2. Verify `editor.formatOnSave` is enabled
3. Check that gofumpt is the formatter in settings

### Tests not discovered

1. Ensure files have `_test.go` suffix
2. Check build tags in settings match test files
3. Refresh Test Explorer
