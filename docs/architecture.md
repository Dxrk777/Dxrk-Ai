# Architecture & Development

← [Back to README](../README.md)

---

## Architecture

```
cmd/dxrk/                  CLI entrypoint
internal/
  app/                     Command dispatch + runtime wiring
  model/                   Domain types (agents, components, skills, presets, personas)
  catalog/                 Registry definitions (agents, skills, components)
  system/                  OS/distro detection, dependency checks, platform guards
  cli/                     Install flags, validation, orchestration, dry-run
  planner/                 Dependency graph, resolution, ordering, review payloads
  installcmd/              Profile-aware command resolver (brew/apt/pacman/dnf/winget/go install)
  pipeline/                Staged execution + rollback orchestration
  backup/                  Config snapshot + restore
  assets/                  Embedded skill files + persona templates
  components/              Per-component install/inject logic
    dxrk-memory/  sdd/  skills/  mcp/  persona/  theme/  permissions/  dxrk-guardian/
    filemerge/             Marker-based file merging (inject without clobbering)
  agents/                  Agent adapters (config strategy per agent)
    claude/  opencode/  gemini/  cursor/  vscode/  codex/  windsurf/  antigravity/
  opencode/                OpenCode model/config parsing utilities
  state/                   Installation state tracking
  update/                  Self-update + upgrade logic
  verify/                  Post-apply health checks + reporting
  tui/                     Bubbletea TUI (Rose Pine theme)
    styles/  screens/
scripts/                   Installer scripts (bash + PowerShell)
e2e/                       Docker-based E2E tests (Ubuntu + Arch)
testdata/                  Golden test fixtures
```

---

## Testing

```bash
# Unit tests (fast, excludes Docker)
go test -short ./...

# Full tests (includes Docker-dependent tests)
go test ./...

# With race detector
go test -race ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

# Linting (golangci-lint v2)
golangci-lint run

# Docker E2E (Ubuntu + Arch, requires Docker)
RUN_FULL_E2E=1 RUN_BACKUP_TESTS=1 ./e2e/docker-test.sh

# Dry-run smoke test (macOS/Linux)
dxrk install --dry-run --agent claude-code --preset minimal

# Dry-run smoke test (Windows PowerShell)
dxrk.exe install --dry-run --agent claude-code --preset minimal
```

Test coverage:

- **73 test packages** across the codebase (all packages pass)
- **~880 test functions** covering all agent adapters, components, LLM providers, sandbox, and system detection
- **~73% average coverage** across the project
- **78 E2E test functions** running in Docker containers (Ubuntu + Arch)
- **17 golden files** for snapshot testing component output
- Full pipeline tested: detection, planning, execution, backup, restore, verification
- All 14 agent adapters have unit tests with cross-platform path validation
- **0 golangci-lint issues** in non-test code, **0 in test code**

---

## Ecosistema Dxrk

| | Dotfiles | Dxrk |
|--|----------|------|
| **Propósito** | Entorno dev (editores, shells, terminales) | Capa de desarrollo IA (agentes, memoria, skills) |
| **Instala** | Neovim, Fish/Zsh, Tmux/Zellij, Ghostty | Configura Claude Code, OpenCode, Gemini CLI, Cursor, VS Code Copilot, Codex, Windsurf, Antigravity |
| **Solape** | Ninguno — complementarios | Ninguno — capa diferente |

Instalá primero los Dotfiles para tu entorno dev, luego Dxrk para la capa de IA encima.

---

## License

MIT
