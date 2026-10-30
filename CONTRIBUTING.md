# Contributing

Thanks for helping out. Small, focused pull requests are the easiest to review.

## Development setup

You need Go (the version in `go.mod` or newer). No C toolchain is required. Node.js 22 or newer is only needed to lint and type-check the browser JavaScript; nothing from npm ships in the binary.

```sh
git clone https://github.com/neverlless/taskshed.git
cd taskshed
go run ./cmd/server        # http://localhost:8080, SQLite file ./taskshed.db
```

Pages are server-rendered with [templ](https://templ.guide) and htmx. Templates live in `internal/ui/*.templ`; templ is pinned in `go.mod` as a Go tool, so no separate install is needed. After editing a `.templ` file, regenerate and commit the `*_templ.go` output:

```sh
go tool templ generate             # once
go tool templ generate --watch     # or keep it running while you work
```

Templates compile to Go and static assets in `web/static` are embedded at build time, so restart the server after editing either. The two JS modules in `web/static/js` use `// @ts-check` and JSDoc types; keep them that way.

To work against PostgreSQL:

```sh
docker run -d --name taskshed-pg -p 5432:5432 \
  -e POSTGRES_USER=taskshed -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=taskshed postgres:18-alpine
DB_TYPE=postgres DB_HOST=localhost DB_USER=taskshed DB_PASSWORD=dev go run ./cmd/server
```

## Tests and lint

Run these before opening a pull request; CI runs the same checks:

```sh
go tool templ generate     # CI fails if generated files are stale or uncommitted
go mod tidy                # CI fails if go.mod or go.sum are not tidy
go vet ./...
go test -race ./...
golangci-lint run          # golangci-lint v2, config in .golangci.yml
npm ci && npm run check    # Biome lint and TypeScript check of web/static/js
```

`npm run format` applies Biome's formatting to the JS.

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
