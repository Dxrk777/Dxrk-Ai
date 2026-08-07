# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [4.1.0] - 2026-08-05

### Added
- Complete ConfigManager: hierarchical config resolution, validation, and typed accessors (`internal/config/`)
- Queue API surface with list/get operations (`internal/task/queue.go`)
- 14 new agent adapters: Aider, Cline, Roo Code, Continue, Junie, Amazon Q, OpenHands, Zed AI, GitHub Copilot, Devin, Cody, Tabnine, Replit, Void (28 total)
- ML training pipeline (`internal/ml/`): data validation, model training, GGUF conversion, Hugging Face push, configurable hyperparameters
- ESLint + Prettier for web UI (`web/`)
- ErrorBoundary component for React crash handling
- SettingsPage write support (POST /api/settings)
- WebSocket hook refactor with connectRef pattern
- Python test coverage reporting in CI (Codecov)
- benchstat integration for benchmark regression detection in CI
- CHANGELOG.md
- CI badge in README
- Hello World section in README
- Roadmap section in README
- Build from source instructions in README
- 10 new MCP servers: context7, kubernetes, redis, mongodb, elasticsearch, jira, grafana, prometheus, openai, confluence (35 total)
- rag-code-mcp and chunkhound MCP servers for RAG workflows
- 9 Go library wrappers: gin, jwt, pgx, redis, fx, cron, gocron, websocket, grpc
- 3 GitHub Actions workflows: security (CodeQL + Trivy), quality (SonarCloud), audit (OWASP + govulncheck)
- Dev tooling: dependabot, CODEOWNERS, PR template, mypy type checking in CI
- Pre-commit hooks: golangci-lint, black, mypy
- 10 new golangci-lint linters: copyloopvar, durationcheck, errname, fatcontext, makezero, mirror, sqlclosecheck, tparallel, usestdlibvars, wastedassign

### Fixed
- 13 TUI golden files regenerated for test compatibility
- 3 sandbox test files gated with `//go:build docker`
- Web UI TypeScript build errors (unused imports/vars)
- CSS `@import` ordering warning
- Phantom sidebar pages (Terminal/Files) removed
- Dashboard/LogsPage WebSocket connection via useWebSocket hook
- TestShowTip case-sensitive match (`Dxrk` → `dxrk`)
- E2E lib.sh hyphenated bash variable name
- Deprecated `tenv` linter removed from golangci-lint config
- Lint issues from new linters (errname, tparallel, usestdlibvars, wastedassign)
- Flaky `TestLearner` in `internal/autonomy` (TempDir cleanup race) fixed by making learner persistence synchronous
- Flaky `TestConcurrentAccess` in `internal/services/ratelimit` made deterministic under `-race` (burst test uses 1 token/s refill to avoid overflow)
- Release brew and scoop publishing disabled until `homebrew-tap` and `scoop-bucket` repos exist

### Changed
- Skills registry expanded to 2,242 total (34 dxrk + 2,208 imported)
- Curated skill pack: 66 skills across 10 categories
- Go 1.25.12 minimum (was incorrectly documented as 1.26)
- golangci-lint config expanded with 10 new linters for code quality
- CI Actions upgraded: checkout v7, setup-go v7, upload-artifact v7, download-artifact v8, CodeQL v4
- Web deps upgraded: vite 8.2.0, lucide-react 1.28.0, @playwright/test 1.62.1, autoprefixer 10.5.4

## [4.0.0] - 2026-07-26

### Added
- Go CLI via Cobra (`cmd/dxrk/`)
- Viper config with env prefix `DXRK`
- Zap structured logging adapter
- Colly web scraper integration
- GoReleaser v2 config (Linux, macOS, Windows; amd64 + arm64)
- Engram REST API (31 endpoints: sessions, observations, search, timeline, prompts, context, export/import, stats, conflicts)
- Engram Cloud API (sync push/pull, HTMX dashboard)
- MCP protocol server (stdio + TCP)
- Web UI (React 18 + Vite 6 + Tailwind 3 + TypeScript strict)
- Docker build + E2E tests (Ubuntu, Arch, Fedora)
- CI pipeline: lint, unit tests, Python tests, full tests, security scan, benchmarks, E2E, Docker build
- PR validation workflow (size limit, issue reference, labels)
- Renovate config with custom regex manager for pinned versions
- SBOM generation via Trivy
- 35 MCP servers (Context7, Kubernetes, Redis, MongoDB, Elasticsearch, Jira, Grafana, Prometheus, OpenAI, Confluence, and more)
- 39 Go benchmarks across 7 packages
- 1,320 Python tests
- 2,242 skills (34 dxrk-specific + 2,208 imported from 4 community collections)
- 66 curated skills across 10 categories
- Install scripts (bash + PowerShell) with checksum verification

### Changed
- Module path: `github.com/Dxrk777/Dxrk`
- Go 1.25.12 minimum
- Python >=3.13 required

## [3.0.0] - 2026-07-01

### Added
- Initial Go rewrite from Python-only architecture
- Internal packages: router, rag, pipeline, sandbox, compress, tools, query, vault, observe, webui, mcp
- React web dashboard

## [2.0.0] - 2026-06-01

### Added
- Python CLI (`dxrk/`)
- Agent skill system
- MCP server integration

## [1.0.0] - 2026-05-01

### Added
- Initial release
- Basic AI agent configuration
