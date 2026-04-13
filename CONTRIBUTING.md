# Contributing

Thanks for contributing to `engineering-model-go`.

## Development Setup

1. Install Go.
2. Clone the repository.
3. Run tests:

```bash
go test ./...
```

## Code Guidelines

- Keep behavior deterministic.
- Keep model parsing and rendering logic explicit and testable.
- Keep dependency additions minimal and justified.
- Prefer small, focused pull requests.

## Pull Requests

Before opening a PR:

1. Run formatting and tests:

```bash
gofmt -w .
go test ./...
go vet ./...
```

2. Update tests for behavior changes.
3. Update README if API behavior changed.
4. Keep sample outputs reproducible from the example project.

## Reporting Issues

Please include:

- minimal reproducible model input
- expected behavior
- actual behavior and diagnostics
