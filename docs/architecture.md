# Architecture & Development

← [Back to README](../README.md)

---

## Architecture

```
cmd/dxrk/                  CLI entrypoint
internal/
  agents/                  Agent adapters (42): claude, opencode, gemini, cursor, vscode,
                           codex, windsurf, antigravity, kilocode, kimi, kiro, qwen, pi,
                           openclaw, aider, cline, roocode, continue_adapter, junie,
                           amazonq, openhands, zedai, copilot, devin, cody, tabnine,
                           replit, void_adapter, trae, v0, qodo, pearai, looperators,
                           lovable, bolt, blackbox, amp, hermes, jetbrains, zcode,
                           runcell, conductor + factory/registry/discovery
  cli/                     Cobra CLI: chat, install, run, query, sync, restore, mcp,
                           validate, dryrun, completion, uninstall
  commands/                Command dispatch (session listing, status, etc.)
  tui/                     Bubbletea TUI (Rose Pine theme)
    styles/  screens/  testdata/ (17 golden files)
  services/                autodream, compact, extractmemories, notifications,
                           policylimits, promptsuggestion, ratelimit, remotesettings,
                           sessionmemory, teammemorysync, tips, tokencount, voice
  utils/                   Shared primitives
    hooks/                 Hook registry/executor/queue/matcher/logger/circuit-breaker
    http/                  HTTP client (retry, proxy, TLS, connection pool)
    image/                 Image format detection/processing (incl. PDF)
    session/               Session model + storage/migration/restore
    swarm/                 Event bus, leader election, coordinator, health monitor
    bashparse/  diff/  fileops/  messages/  permissions/
  components/              Per-component install/inject logic
    dxrkmemory/  sdd/  skills/  mcp/  persona/  theme/  permissions/
    dxrkguardian/  filemerge/  checker/  internalmcp/  opencodeplugin/  uninstall/
  security/                AST-based scanning, policy enforcement
  plugin/                  Plugin policy management (allow/block lists, timeouts)
  model/                   Domain types (agents, components, skills, presets, personas)
  catalog/                 Registry definitions (agents, skills, components)
  system/                  OS/distro detection, dependency checks, platform guards
  planner/                 Dependency graph, resolution, ordering, review payloads
  installcmd/              Profile-aware command resolver (brew/apt/pacman/dnf/winget/go install)
  pipeline/                Staged execution + rollback orchestration
  backup/                  Config snapshot + restore
  state/  update/  verify/ Installation state, self-update, post-apply checks
  app/  assets/  config/  coordinator/  cost/  costopt/  db/  devex/  di/  dr/  git/
  log/  marketplace/  mcp/  memory/  ml/  multitenant/  observe/  opencode/  query/
  rag/  remote/  resilience/  router/  rpc/  sandbox/  schedule/  scraper/  skillregistry/
  slo/  task/  tasks/  team/  telemetry/  tools/  trace/  vault/  version/  versions/
  web/  webui/  workspace/  ws/  a2a/  agentbuilder/  auth/  autonomy/  bridge/  cache/
  chaos/  compress/
scripts/                   Installer scripts (bash + PowerShell)
e2e/                       Docker-based E2E tests (Ubuntu + Arch)
testdata/                  Golden test fixtures
gateway/  tui_gateway/  web/  plugins/  skills/  dxrk/ (Python)  tools/  agent/
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
- All 42 agent adapters have unit tests with cross-platform path validation
- **0 golangci-lint issues** in non-test code, **0 in test code**
- Core util suites verified: `internal/utils/{hooks,http,image,swarm}` (registry/executor, retry/proxy/TLS, format detection, event bus + leader election) and `internal/{tui,cli}` (UI screens, CLI install/run/sync/restore)

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
