# Development Guide

This document provides information about the development workflow, code quality tools, and best practices for the Loom project.

## 🚀 Quick Start

1. **Install Go tools:**
   ```bash
   make install-tools
   ```

2. **Install pre-commit (choose one):**
   ```bash
   # macOS with Homebrew
   brew install pre-commit

   # Using pip
   pip install pre-commit

   # Using conda
   conda install -c conda-forge pre-commit
   ```

3. **Set up pre-commit hooks:**
   ```bash
   make install-pre-commit
   ```

4. **Run initial setup:**
   ```bash
   make dev-setup
   ```

## 🛠️ Available Commands

Use `make help` to see all available commands:

```bash
make help
```

### Common Development Tasks

- **`make dev-setup`** - Complete development environment setup
- **`make check`** - Run all code quality checks (lint, vet, test)
- **`make test`** - Run tests
- **`make fmt`** - Format code
- **`make lint`** - Run linter
- **`make build`** - Build the binary
- **`make clean`** - Clean build artifacts

### Advanced Commands

- **`make test-race`** - Run tests with race detection
- **`make coverage`** - Generate test coverage report
- **`make benchmark`** - Run benchmarks
- **`make security`** - Run security vulnerability checks
- **`make pre-commit`** - Run all pre-commit hooks

## 🔍 Code Quality Tools

### Linting with golangci-lint

The project uses [golangci-lint](https://golangci-lint.run/) with a comprehensive configuration:

- **Enabled linters:** 40+ linters including staticcheck, govet, errcheck, gosec, and more
- **Configuration:** `.golangci.yml`
- **Run manually:** `make lint` or `golangci-lint run`

### Pre-commit Hooks

Pre-commit hooks run automatically on every commit to ensure code quality:

- **Go formatting** with `gofmt` and `goimports`
- **Linting** with `golangci-lint`
- **Testing** with `go test`
- **Module tidying** with `go mod tidy`
- **Security scanning** with `detect-secrets`
- **General checks** (trailing whitespace, YAML validation, etc.)

### Security Scanning

- **`govulncheck`** - Go vulnerability database scanner
- **`detect-secrets`** - Secrets detection
- **`gosec`** - Go security analyzer (via golangci-lint)

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run tests in short mode
make test-short

# Generate coverage report
make coverage
```

### Test Coverage

The project maintains test coverage tracking:
- Coverage reports are generated in `coverage.html`
- CI uploads coverage to Codecov
- Aim for >80% coverage on new code

## 🏗️ Building

### Local Development

```bash
# Build for current platform
make build

# Build for all platforms
make build-all
```

### Release Builds

The CI/CD pipeline automatically builds binaries for multiple platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## 📋 Git Workflow

### Commit Guidelines

1. **Pre-commit hooks** run automatically and must pass
2. **Commit messages** should be clear and descriptive
3. **Small, focused commits** are preferred
4. **Test your changes** before committing

### Branch Protection

The `main` branch is protected and requires:
- All status checks to pass
- No merge commits (rebase preferred)
- Up-to-date branches

## 🤖 CI/CD Pipeline

### GitHub Actions Workflows

The project uses comprehensive CI/CD with multiple jobs:

1. **Code Quality** - Linting, formatting, and static analysis
2. **Security Scan** - Vulnerability and security checks
3. **Testing** - Cross-platform testing on Go 1.21, 1.22, 1.23
4. **Benchmarks** - Performance regression detection
5. **Build** - Multi-platform binary compilation
6. **Dependency Review** - Automated dependency scanning

### Status Checks

All PRs must pass:
- ✅ Code quality checks
- ✅ Security scans
- ✅ All tests across platforms
- ✅ Benchmarks (performance)
- ✅ Dependency review

## 🔧 Configuration Files

| File | Purpose |
|------|---------|
| `.golangci.yml` | golangci-lint configuration |
| `.pre-commit-config.yaml` | Pre-commit hooks configuration |
| `.secrets.baseline` | detect-secrets baseline |
| `.github/workflows/ci.yml` | GitHub Actions CI/CD pipeline |
| `Makefile` | Development commands and automation |

## 🐛 Troubleshooting

### Common Issues

**Pre-commit hooks failing:**
```bash
# Update pre-commit hooks
pre-commit autoupdate

# Run hooks manually
pre-commit run --all-files
```

**Linter errors:**
```bash
# Auto-fix formatting issues
make fmt

# Check specific linter issues
golangci-lint run --enable-only=<linter-name>
```

**Test failures:**
```bash
# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -v ./path/to/package -run TestName
```

### Getting Help

1. Check this documentation
2. Review tool-specific documentation:
   - [golangci-lint docs](https://golangci-lint.run/)
   - [pre-commit docs](https://pre-commit.com/)
3. Check existing issues and discussions
4. Create a new issue with reproduction steps

## 📚 Additional Resources

- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Testing](https://go.dev/doc/tutorial/add-a-test)
- [golangci-lint Configuration](https://golangci-lint.run/usage/configuration/)
