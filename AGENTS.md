# Repository Guidelines

## Project Structure & Module Organization
Core backend code is at the repository root as a single Go module (`module github.com/deepch/RTSPtoWeb`).
- Entry point: `RTSPtoWeb.go`.
- HTTP/API handlers: `apiHTTP*.go`.
- Streaming/runtime pipeline: `stream*.go`, `serverRTSP.go`, `hls*.go`.
- State/config handling: `storage*.go`, `storageConfig.go`, `loggingLog.go`.
- Web UI: `web/templates/` and `web/static/`.
- Docs and browser examples: `docs/` and `docs/examples/`.

Use `config.json` for local runtime settings; keep sensitive overrides in a separate untracked file.

## Build, Test, and Development Commands
- `make build`: compile the binary from `*.go`.
- `make run`: run directly with `go run *.go`.
- `make server SERVER_FLAGS="-config config.json"`: run the built binary with explicit flags.
- `make lint`: run `go vet` checks.
- `make test`: run `test.curl` and `test_multi.curl` API smoke scripts.
- `docker build -t rtsp-to-web .`: build the local container image (release image is published via `.github/workflows/publish-docker.yml`).

`make test` expects a running server on `127.0.0.1:8083` and uses demo HTTP auth credentials.

## Coding Style & Naming Conventions
Use standard Go formatting and imports:
- Run `gofmt -w *.go` before committing.
- Keep file names domain-oriented with existing prefixes (`apiHTTP`, `storage`, `stream`, `hls`).
- Use PascalCase for exported identifiers and camelCase for local vars.

Follow existing JSON/config key style in `config.json` when adding new fields.

## Testing Guidelines
This repo currently relies on smoke tests and static checks:
- `go vet` via `make lint`.
- HTTP API exercises via `make test`.

There are no `*_test.go` unit tests yet; add them for new non-trivial logic and name them `<feature>_test.go`.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit patterns, especially:
- `fix(deps): ...`
- `chore(deps): ...`
- `fix: ...`

Use concise, scoped commit subjects and keep changes focused.
For PRs, include:
- purpose and behavior change summary
- config/API impact (`config.json`, `docs/api.md`) if applicable
- verification steps run (`make lint`, `make test`, manual UI/API checks)
- screenshots only when UI under `web/` changes

## Security & Configuration Tips
- Do not commit real camera URLs, credentials, or private certificates/keys.
- Prefer `-config /path/to/local.json` for environment-specific secrets.
- Review `SECURITY.md` before reporting or handling vulnerabilities.
