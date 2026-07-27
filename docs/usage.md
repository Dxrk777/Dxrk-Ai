# Usage

← [Back to README](../README.md)

---

## Persona Modes

| Persona   | ID          | Description                                                                       |
| --------- | ----------- | --------------------------------------------------------------------------------- |
| Dxrk | `dxrk` | Teaching-oriented mentor persona — pushes back on bad practices, explains the why |
| Neutral   | `neutral`   | Same teacher, same philosophy, no regional language — warm and professional       |
| Custom    | `custom`    | Keep your existing persona/config unmanaged — dxrk does not inject a persona |

`custom` is a compatibility/ownership choice, not a persona editor. Use it when you already have your own persona instructions and want dxrk to leave them alone.

---

## Interactive TUI

Just run it — the Bubbletea TUI guides you through agent selection, components, skills, presets, and managed uninstall flows:

```bash
dxrk
```

The uninstall flow is also available from the TUI menu. It lets you:

- select one or more configured agents
- select which managed components to remove (for example `sdd`, `persona`, or `context7`)
- confirm the exact uninstall scope before applying changes

Before any managed file is modified, `dxrk` creates a backup snapshot so the configuration can be restored later if needed.

---

## CLI Commands

### install

First-time setup — detects your tools, configures agents, injects all components:

```bash
# Full ecosystem for multiple agents
dxrk install \
  --agent claude-code,opencode,gemini-cli \
  --preset full-dxrk

# Minimal setup for Cursor
dxrk install \
  --agent cursor \
  --preset minimal

# OpenClaw setup after installing OpenClaw manually
dxrk install \
  --agent openclaw \
  --preset full-dxrk

# Pick specific components and skills
dxrk install \
  --agent claude-code \
  --component dxrk-memory,sdd,skills,context7,persona,permissions \
  --skill go-testing,skill-creator,branch-pr,issue-creation \
  --persona dxrk

# Dry-run first (preview plan without applying changes)
dxrk install --dry-run \
  --agent claude-code,opencode \
  --preset full-dxrk
```

### skill-registry refresh

Refresh the project-local skill registry used by orchestrators before they delegate work:

```bash
dxrk skill-registry refresh
dxrk skill-registry refresh --force
dxrk skill-registry refresh --cwd /path/to/project --quiet
```

The command scans project skills first (`skills/`, `.opencode/skills/`, `.claude/skills/`, `.github/skills/`, and other supported workspace skill roots), then global agent skill directories. Project-local skills win over same-name global skills.

The command writes `.atl/skill-registry.md` and `.atl/.skill-registry.cache.json`. The cache fingerprint includes schema version plus each discovered `SKILL.md` file path, mtime, and size, so normal startup is a cheap cache-hit when skills have not changed.

Claude Code and OpenCode installs wire this command into startup/plugin hooks. Pi gets the equivalent behavior from `dxrk-pi`; keep that extension's scan roots in sync when changing these discovery rules.

### sync

Refresh managed assets to the current version. Use after `brew upgrade dxrk` or when you want your local configs aligned with the latest release. Does NOT reinstall binaries (dxrk-memory, Dxrk-Guardian) — only updates prompt content, skills, MCP configs, and SDD orchestrators.

```bash
# Sync all installed agents
dxrk sync

# Sync specific agents only
dxrk sync --agent cursor --agent windsurf

# Sync a specific component
dxrk sync --component sdd
dxrk sync --component skills
dxrk sync --component dxrk-memory

# Refresh OpenClaw workspace instructions and MCP config
dxrk sync --agent openclaw
```

Sync is safe and idempotent — running it twice produces no changes the second time.

For OpenClaw, sync reads the active workspace from `~/.openclaw/openclaw.json` (`agents.defaults.workspace`). It writes `AGENTS.md` / `SOUL.md` into that workspace, while MCP servers stay in the global OpenClaw config under `mcp.servers`.

### uninstall

Remove only the `dxrk` managed configuration from one or more agents. This does not uninstall external packages or binaries — it removes managed prompt sections, MCP entries, skills/config fragments, and other managed files, then updates `state.json` accordingly.

Before any change is applied, `dxrk` creates a backup snapshot of the affected files.

```bash
# Partial uninstall for specific agents
dxrk uninstall \
  --agent claude-code \
  --agent opencode

# Partial uninstall for specific components only
dxrk uninstall \
  --agent claude-code \
  --component sdd,persona,context7

# Complete uninstall of managed config from all supported agents
dxrk uninstall --all

# Skip confirmation prompt
dxrk uninstall --agent cursor --component skills --yes
```

If no `--component` flag is provided for a partial uninstall, `dxrk` removes all managed uninstallable components for the selected agent set.

### update / upgrade

Check for and install new versions of `dxrk` itself:

```bash
# Check if a newer version is available
dxrk update

# Upgrade to the latest release (downloads new binary, replaces current)
dxrk upgrade
```

After upgrading, run `dxrk sync` to refresh all managed assets to the new version's content.

If GitHub rate-limits update checks, export `GITHUB_TOKEN` or `GH_TOKEN` before running `dxrk update`/`upgrade`.

### version

```bash
dxrk version
dxrk --version
dxrk -v
```

---

## CLI Flags (install)

| Flag                          | Description                                                                                                       |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `--agent`, `--agents`         | Agents to configure (comma-separated)                                                                             |
| `--component`, `--components` | Components to install (comma-separated)                                                                           |
| `--skill`, `--skills`         | Skills to install (comma-separated)                                                                               |
| `--persona`                   | Persona mode: `dxrk`, `neutral`, `custom` (`custom` keeps your existing persona unmanaged)                   |
| `--preset`                    | Preset: `full-dxrk`, `ecosystem-only`, `minimal`, `custom` (`custom` means manual component/skill selection) |
| `--dry-run`                   | Preview the install plan without applying changes                                                                 |

## CLI Flags (sync)

| Flag                     | Description                                                                                          |
| ------------------------ | ---------------------------------------------------------------------------------------------------- |
| `--agent`, `--agents`    | Agents to sync (defaults to all installed agents)                                                    |
| `--component`            | Sync a specific component only: `sdd`, `dxrk-memory`, `context7`, `skills`, `dxrk-guardian`, `permissions`, `theme` |
| `--profile`              | Create or update an SDD profile: `name:provider/model` (sets the default model for all phases)       |
| `--profile-phase`        | Override a specific phase in a profile: `name:phase:provider/model`                                  |
| `--sdd-profile-strategy` | OpenCode profile sync strategy: `generated-multi` or `external-single-active`                        |
| `--include-permissions`  | Include permissions sync (opt-in)                                                                    |
| `--include-theme`        | Include theme sync (opt-in)                                                                          |

**Profile examples:**

```bash
# Create a "cheap" profile using a free model for all phases
dxrk sync --profile cheap:openrouter/qwen/qwen3-30b-a3b:free

# Override the design phase to use a stronger model
dxrk sync --profile-phase cheap:sdd-design:anthropic/claude-sonnet-4-20250514

# Create multiple profiles in one command
dxrk sync \
  --profile cheap:openrouter/qwen/qwen3-30b-a3b:free \
  --profile premium:anthropic/claude-sonnet-4-20250514

# Use compatibility mode with an external OpenCode profile manager
dxrk sync --agent opencode --sdd-profile-strategy external-single-active
```

See [OpenCode SDD Profiles](opencode-profiles.md) for the full guide.

## CLI Flags (uninstall)

| Flag                          | Description                                                             |
| ----------------------------- | ----------------------------------------------------------------------- |
| `--agent`, `--agents`         | Agents to uninstall managed config from (required unless using `--all`) |
| `--component`, `--components` | Managed components to remove only from the selected agents              |
| `--all`                       | Remove managed configuration from all supported agents                  |
| `--yes`, `-y`                 | Skip the confirmation prompt                                            |

---

## Typical Workflow

```bash
# First time: install everything
brew install dxrk-programming/tap/dxrk
dxrk install --agent claude-code,cursor --preset full-dxrk

# After a new release: upgrade + sync
brew upgrade dxrk
dxrk sync

# Remove only managed SDD + persona config from one agent
dxrk uninstall --agent claude-code --component sdd,persona

# Adding a new agent later
dxrk install --agent windsurf --preset full-dxrk
```

---

## Dependency Management

`dxrk` auto-detects prerequisites before installation and provides platform-specific guidance:

- **Detected tools**: git, curl, node, npm, brew, go
- **Version checks**: validates minimum versions where applicable
- **Platform-aware hints**: suggests `brew install`, `apt install`, `pacman -S`, `dnf install`, or `winget install` depending on your OS
- **Node LTS alignment**: on apt/dnf systems, Node.js hints use NodeSource LTS bootstrap before package install
- **Dependency-first approach**: detects what's installed, calculates what's needed, shows the full dependency tree before installing anything, then verifies each dependency after installation
