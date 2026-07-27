# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- ESLint + Prettier for web UI (`web/`)
- ErrorBoundary component for React crash handling
- SettingsPage write support (POST /api/settings)
- WebSocket hook refactor with connectRef pattern
- Python test coverage reporting in CI (Codecov)
- benchstat integration for benchmark regression detection in CI
- CHANGELOG.md

### Fixed
- 13 TUI golden files regenerated for test compatibility
- 3 sandbox test files gated with `//go:build docker`
- Web UI TypeScript build errors (unused imports/vars)
- CSS `@import` ordering warning
- Phantom sidebar pages (Terminal/Files) removed
- Dashboard/LogsPage WebSocket connection via useWebSocket hook

### Changed
- Skills registry expanded to 2,242 total (34 dxrk + 2,208 imported)
- Curated skill pack: 66 skills across 10 categories

## [4.0.0] - 2026-07-26

### Added
- Go CLI via Cobra (`cmd/dxrk/`)
- Viper config with env prefix `DXRK`
- Zap structured logging adapter
- Colly web scraper integration
- GoReleaser v2 config (Linux, macOS, Windows; amd64 + arm64)
- Homebrew tap + Scoop bucket publishing
- Engram REST API (31 endpoints: sessions, observations, search, timeline, prompts, context, export/import, stats, conflicts)
- Engram Cloud API (sync push/pull, HTMX dashboard)
- MCP protocol server (stdio + TCP)
- Web UI (React 18 + Vite 6 + Tailwind 3 + TypeScript strict)
- Docker build + E2E tests (Ubuntu, Arch, Fedora)
- CI pipeline: lint, unit tests, Python tests, full tests, security scan, benchmarks, E2E, Docker build
- PR validation workflow (size limit, issue reference, labels)
- Renovate config with custom regex manager for pinned versions
- SBOM generation via Trivy
- 39 Go benchmarks across 7 packages
- 1,320 Python tests
- 2,242 skills (34 dxrk-specific + 2,208 imported from 4 community collections)
- 66 curated skills across 10 categories
- Install scripts (bash + PowerShell) with checksum verification

### Changed
- Module path: `github.com/Dxrk777/Dxrk-Ai
- Go 1.26.0 minimum
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
