# Contributing to Orchestrator

Thank you for your interest in contributing to orchestrator! This project is maintained by [ProxySQL](https://proxysql.com) and we welcome contributions from the community.

## How to Contribute

### Reporting Bugs

- Use [GitHub Issues](https://github.com/proxysql/orchestrator/issues) with the **Bug Report** template
- Include your orchestrator version, backend type (MySQL/SQLite), and sanitized configuration
- Provide topology information: `orchestrator-client -c topology -alias my-cluster`
- Include logs with `--debug --stack` flags for maximum verbosity

### Suggesting Features

- Use [GitHub Issues](https://github.com/proxysql/orchestrator/issues) with the **Feature Request** template
- Describe the use case and proposed solution
- Discuss your idea in an issue before starting a PR

### Submitting Pull Requests

1. Fork the repository
2. Create a feature branch from `master`
3. Make your changes
4. Submit a PR against `master`
5. Reference the related issue in the PR description

## Coding Standards

- Format code with `gofmt -s` (do **not** use `goimports`)
- Follow existing code conventions and patterns
- Add tests for new functionality
- Ensure all CI checks pass:
  - Code formatting (`gofmt`)
  - Build verification
  - Unit tests
  - Integration tests (MySQL and SQLite backends)
  - Documentation validation

## Developer Certificate of Origin (DCO)

All contributions must be signed off under the [Developer Certificate of Origin](https://developercertificate.org/) (DCO). This is a lightweight mechanism to certify that you wrote or have the right to submit the code you are contributing.

To sign off, add a `Signed-off-by` line to your commit messages:

```
Signed-off-by: Your Name <your.email@example.com>
```

Git can do this automatically with the `-s` flag:

```bash
git commit -s -m "Your commit message"
```

## Building and Testing

```bash
# Build
./script/build

# Run unit tests
go test ./go/...

# Run a single package's tests
go test ./go/inst/...

# Run a specific test
go test ./go/inst/... -run TestBinlogCoordinates
```

### Using the Makefile

The project includes a root-level Makefile for common tasks:

```bash
make build      # Build bin/orchestrator
make test       # Run unit tests
make lint       # Run golangci-lint
make fmt        # Format code with gofmt
make check-fmt  # Check formatting (CI)
make clean      # Remove build artifacts
```

See [docs/build.md](docs/build.md) for detailed build and test instructions.

## Code Review

All submissions require review before merging. Maintainers may request changes or suggest improvements. Please be patient — we aim to review PRs promptly but availability varies.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
