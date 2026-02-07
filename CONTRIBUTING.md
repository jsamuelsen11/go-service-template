# Contributing to Go Service Template

Thank you for your interest in contributing! This guide covers the development workflow,
code standards, and processes for submitting changes.

## Table of Contents

- [Quick Start](#quick-start)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Git Hooks](#git-hooks)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Testing Requirements](#testing-requirements)
- [Related Documentation](#related-documentation)

---

## Quick Start

Follow the setup instructions in the [README](README.md#installation):

```bash
# Install prerequisites
curl https://mise.run | sh
mise use go@1.25.7
brew install go-task

# Clone and Set Up
git clone <repository-url>
cd go-service-template
task setup  # Installs dependencies and git hooks

# Start development
task run    # Hot reload server on port 8080
```

---

## Development Workflow

### Daily Development Loop

1. **Create a feature branch:**

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes** following the [architecture patterns](docs/ARCHITECTURE.md)

3. **Run tests locally:**

   ```bash
   task test              # Unit tests with race detection
   task lint              # Linting
   task ci                # Full CI pipeline (fmt, lint, test, coverage, vuln)
   ```

4. **Commit your changes** (hooks run automatically):

   ```bash
   git add .
   git commit -m "feat(scope): add new feature"
   ```

5. **Push and create PR:**

   ```bash
   git push -u origin feature/your-feature-name
   ```

### Key Commands

| Command                       | Description                        |
| ----------------------------- | ---------------------------------- |
| `task run`                    | Start server with hot reload       |
| `task test`                   | Run unit tests with race detection |
| `task test:integration:smoke` | Run BDD integration tests (smoke)  |
| `task lint`                   | Run golangci-lint                  |
| `task fmt`                    | Format code with gofumpt           |
| `task ci`                     | Full CI pipeline locally           |
| `task coverage`               | Generate coverage report           |
| `task generate`               | Regenerate mocks                   |

---

## Code Standards

### Architecture

This project uses **Hexagonal Architecture** (Ports and Adapters). Follow these principles:

- **Domain layer** contains pure business logic with no external dependencies
- **Ports** define interfaces for inbound and outbound operations
- **Adapters** implement ports for specific technologies (HTTP, databases, etc.)
- **Dependencies flow inward** toward the domain layer

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for complete details.

### Formatting

- Code is formatted with **gofumpt** (stricter than gofmt)
- Formatting runs automatically on commit via git hooks
- Run manually: `task fmt`

### Linting

- **golangci-lint** runs 22+ linters configured in `.golangci.yml`
- Key linters: errcheck, govet, staticcheck, gosec, revive, gocyclo
- Complexity limits: cyclomatic (15), cognitive (20), nesting (5)
- Run manually: `task lint`

### Patterns

Follow established patterns for:

- Error handling and wrapping
- Context propagation
- Concurrency patterns
- Service layer design

See [docs/PATTERNS.md](docs/PATTERNS.md) for examples.

---

## Git Hooks

This project uses [Lefthook](https://github.com/evilmartians/lefthook) for Git hooks.
Hooks are installed automatically when you run `task setup`.

### Pre-commit Hooks

These run in parallel when you commit:

| Hook          | Files                       | Purpose                       | Auto-fix |
| ------------- | --------------------------- | ----------------------------- | -------- |
| golangci-lint | `*.go`                      | Lint with 22+ linters         | Yes      |
| gofumpt       | `*.go`                      | Strict Go formatting          | Yes      |
| go-mod-tidy   | `*.go`                      | Ensure go.mod is tidy         | Yes      |
| gitleaks      | all staged                  | Detect secrets in commits     | No       |
| actionlint    | `.github/workflows/*.yml`   | Lint GitHub Actions workflows | No       |
| vacuum        | `*openapi*.yaml`            | Validate OpenAPI specs        | No       |
| markdownlint  | `*.md`                      | Lint markdown files           | No       |
| misspell      | `*.go,md,yaml,json,toml,sh` | Fix spelling errors           | Yes      |
| prettier      | `*.yaml,yml,json,md`        | Format config files           | Yes      |
| shellcheck    | `*.sh`                      | Lint shell scripts            | No       |
| hadolint      | `Dockerfile*`               | Lint Dockerfiles              | No       |
| tomll         | `*.toml`                    | Format TOML files             | Yes      |

### Commit Message Hook

The `commit-msg` hook validates your commit message using **commitlint**:

- Must follow [Conventional Commits](https://www.conventionalcommits.org/) format
- Format: `type(scope): description`
- See [Commit Messages](#commit-messages) section for details

### Pre-push Hooks

Before pushing, these run in parallel:

| Hook     | Purpose                           |
| -------- | --------------------------------- |
| go-test  | Run all unit tests with gotestsum |
| go-build | Verify the project compiles       |

### Skipping Hooks

In rare cases where you need to bypass hooks (e.g., WIP commits to feature branch):

```bash
# Skip all hooks for this commit
LEFTHOOK=0 git commit -m "wip: work in progress"

# Skip all hooks for this push
LEFTHOOK=0 git push
```

**Warning:** Only skip hooks when absolutely necessary. Never skip hooks when
committing directly to main or when preparing a PR for review.

### Running Hooks Manually

```bash
# Run pre-commit checks
go tool lefthook run pre-commit

# Run pre-push checks
go tool lefthook run pre-push

# Debug with verbose output
LEFTHOOK_VERBOSE=1 git commit -m "message"
```

### Reinstalling Hooks

If hooks stop working after pulling changes:

```bash
go tool lefthook install
```

### Common Hook Issues

| Issue                           | Cause                      | Solution                                       |
| ------------------------------- | -------------------------- | ---------------------------------------------- |
| Hooks not running               | Not installed              | Run `task setup` or `go tool lefthook install` |
| gitleaks blocking commit        | Secrets detected in code   | Remove secrets, use environment variables      |
| markdownlint errors             | Markdown formatting issues | Fix issues or check `.markdownlint.json`       |
| commitlint rejects message      | Wrong format               | Use `type(scope): description` format          |
| hadolint fails                  | Docker not running         | Start Docker or use `LEFTHOOK=0`               |
| prettier conflicts with gofumpt | Both formatting same file  | Prettier excludes `.go` files by design        |

---

## Commit Messages

### Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
type(scope): description

[optional body]

[optional footer(s)]
```

### Types

| Type       | Description                                         |
| ---------- | --------------------------------------------------- |
| `feat`     | New feature                                         |
| `fix`      | Bug fix                                             |
| `docs`     | Documentation changes                               |
| `style`    | Code style (formatting, no logic change)            |
| `refactor` | Code change that neither fixes bug nor adds feature |
| `perf`     | Performance improvement                             |
| `test`     | Adding or updating tests                            |
| `chore`    | Maintenance tasks (deps, configs)                   |
| `ci`       | CI/CD changes                                       |
| `build`    | Build system changes                                |

### Examples

**Good:**

```bash
feat(api): add user registration endpoint
fix(auth): handle expired JWT tokens correctly
docs(readme): update installation instructions
refactor(domain): extract validation to separate service
test(handlers): add integration tests for health endpoint
```

**Bad:**

```bash
updated code                    # No type, vague description
fix stuff                       # No scope, unclear
FEAT: Add feature               # Wrong case, missing scope
feat(api) add endpoint          # Missing colon
```

---

## Pull Request Process

### Before Submitting

Ensure your changes pass all checks:

- [ ] `task test` passes (unit tests)
- [ ] `task lint` passes (no linting errors)
- [ ] `task test:integration:smoke` passes (if applicable)
- [ ] New code has appropriate test coverage
- [ ] Documentation updated (if adding features)

### Branch Naming

Use descriptive branch names:

```text
feature/add-user-authentication
fix/handle-nil-pointer-in-handler
docs/update-api-documentation
refactor/simplify-config-loading
```

### PR Title

PR titles should follow the same format as commit messages:

```text
feat(api): add user registration endpoint
```

### PR Description

Include:

1. **Summary** of what the PR does
2. **Motivation** for the change
3. **Testing** done to verify the change
4. **Breaking changes** (if any)

### Review Process

1. Create PR against `main` branch
2. Ensure CI checks pass
3. Request review from maintainers
4. Address review feedback
5. Squash and merge when approved

---

## Testing Requirements

### Test Coverage

- New features should include unit tests
- Bug fixes should include regression tests
- Aim for meaningful coverage, not arbitrary percentages

### Test Types

| Type        | Command                       | Purpose                     |
| ----------- | ----------------------------- | --------------------------- |
| Unit        | `task test`                   | Test individual components  |
| Integration | `task test:integration:smoke` | Test component interactions |
| Benchmark   | `task test:benchmark`         | Performance measurements    |
| Load        | `task test:load`              | Stress testing with k6      |

### Writing Tests

See the playbook guides for detailed instructions:

- [Writing Unit Tests](docs/playbook/writing-unit-tests.md)
- [Writing Integration Tests](docs/playbook/writing-integration-tests.md)
- [Writing Benchmark Tests](docs/playbook/writing-benchmark-tests.md)
- [Writing Load Tests](docs/playbook/writing-load-tests.md)

---

## Related Documentation

| Document                                   | Description                                            |
| ------------------------------------------ | ------------------------------------------------------ |
| [Architecture](docs/ARCHITECTURE.md)       | System design, layer descriptions, middleware pipeline |
| [Patterns](docs/PATTERNS.md)               | Go patterns for concurrency, services, error handling  |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | Diagnosing and resolving common issues                 |
| [IDE Setup](docs/IDE_SETUP.md)             | VS Code and GoLand configuration                       |
| [Playbook](docs/playbook/README.md)        | Step-by-step how-to guides for common tasks            |
| [ADRs](docs/adr/README.md)                 | Architecture Decision Records                          |
