# Tests for pprof-analyzer-mcp

This directory contains tests for the pprof-analyzer-mcp tool.

## Directory Structure

- `analyzer/`: Tests for the analyzer package
  - `allocs_test.go`: Tests for the allocation profile analysis
  - `flamegraph_test.go`: Tests for flame graph generation
  - `heap_test.go`: Tests for heap profile analysis
  - `memory_leak_test.go`: Tests for memory leak detection

## Running Tests

To run all tests:

```bash
go test -v ./tests/...
```

To run a specific test:

```bash
go test -v ./tests/analyzer -run TestAnalyzeHeapProfile
```

## Test Coverage

To run tests with coverage:

```bash
go test -v -coverprofile=coverage.out ./tests/...
go tool cover -html=coverage.out
```

## Adding New Tests

When adding new functionality to the codebase, please add corresponding tests in this directory. Follow these guidelines:

1. Create a new test file if testing a new feature
2. Use table-driven tests where appropriate
3. Test both normal cases and edge cases
4. Include tests for error conditions
