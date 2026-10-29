# Contributing

Thanks for helping out. Small, focused pull requests are the easiest to review.

## Development setup

You need Go (the version in `go.mod` or newer). No C toolchain is required.

```sh
git clone https://github.com/neverlless/taskshed.git
cd taskshed
go run ./cmd/server        # http://localhost:8080, SQLite file ./taskshed.db
```

The web UI lives in `web/` and is embedded into the binary at build time, so restart the server after editing it.

To work against PostgreSQL:

```sh
docker run -d --name taskshed-pg -p 5432:5432 \
  -e POSTGRES_USER=taskshed -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=taskshed postgres:18-alpine
DB_TYPE=postgres DB_HOST=localhost DB_USER=taskshed DB_PASSWORD=dev go run ./cmd/server
```

## Tests and lint

```sh
go test -race ./...
golangci-lint run          # golangci-lint v2, config in .golangci.yml
go mod tidy                # CI fails if go.mod or go.sum are not tidy
```

## Commits

- Imperative subject line, under about 70 characters: "Add CSV export", not "Added CSV export".
- A short body explaining why, when it is not obvious.
- One logical change per commit.

## Pull requests

1. Fork and create a branch from `main`.
2. Make the change, with tests for new behavior.
3. Update the README if configuration, API or behavior changed.
4. Open a pull request and fill in the template. CI must pass before merge.

Schema changes need a migration that upgrades existing SQLite and PostgreSQL databases in place.
