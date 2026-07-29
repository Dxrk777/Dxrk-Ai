# Supported Agents

← [Back to README](../README.md)

---

## Agent Matrix

| Agent           | ID               | Skills       | MCP | Delegation                       | Output Styles | Slash Commands | Config Path                         |
| --------------- | ---------------- | ------------ | --- | -------------------------------- | ------------- | -------------- | ----------------------------------- |
| Claude Code     | `claude-code`    | Yes          | Yes | Full (Task tool)                 | Yes           | No             | `~/.claude`                         |
| OpenCode        | `opencode`       | Yes          | Yes | Full (multi-mode overlay)        | No            | Yes            | `~/.config/opencode`                |
| Kilo Code       | `kilocode`       | Yes          | Yes | Full (multi-mode overlay)        | No            | Yes            | `~/.config/kilo`                    |
| Gemini CLI      | `gemini-cli`     | Yes          | Yes | Full (experimental)              | No            | No             | `~/.gemini`                         |
| Cursor          | `cursor`         | Yes          | Yes | Full (native subagents)          | No            | No             | `~/.cursor`                         |
| VS Code Copilot | `vscode-copilot` | Yes          | Yes | Full (runSubagent)               | No            | No             | `~/.copilot` + VS Code User profile |
| Codex           | `codex`          | Yes          | Yes | Solo-agent                       | No            | No             | `~/.codex`                          |
| Windsurf        | `windsurf`       | Yes (native) | Yes | Solo-agent                       | No            | No             | `~/.codeium/windsurf`               |
| Antigravity     | `antigravity`    | Yes (native) | Yes | Solo-agent + Mission Control     | No            | No             | `~/.gemini/antigravity`             |
| Kimi Code       | `kimi`           | Yes          | Yes | Full (native custom agents)      | No            | No             | `~/.kimi`                           |
| Qwen Code       | `qwen-code`      | Yes          | Yes | Full (native sub-agents)         | No            | Yes            | `~/.qwen`                           |
| Kiro IDE        | `kiro-ide`       | Yes          | Yes | Full (native subagents)          | No            | No             | `~/.kiro`                           |
| OpenClaw        | `openclaw`       | Yes          | Yes | Solo-agent                       | No            | No             | `~/.openclaw`                       |
| Pi              | `pi`             | Yes          | Yes | Full (package-managed subagents) | No            | Yes            | `~/.pi`                             |
| Aider           | `aider`          | Yes          | Yes | Solo-agent (CLI)                 | No            | No             | `~/.aider.conf.yml`                 |
| Cline           | `cline`          | Yes          | Yes | Full (VS Code ext)               | No            | No             | `~/.cline`                          |
| Roo Code        | `roo-code`       | Yes          | Yes | Full (VS Code ext)               | No            | No             | `~/.roo`                            |
| Continue        | `continue`       | Yes          | Yes | Full (multi-IDE)                 | No            | No             | `~/.continue`                       |
| Junie           | `junie`          | Yes          | Yes | Full (JetBrains)                 | No            | No             | `~/.junie`                          |
| Amazon Q        | `amazon-q`       | Yes          | Yes | Full (AWS ext)                   | No            | No             | `~/.aws/q`                          |
| OpenHands       | `openhands`      | Yes          | Yes | Solo-agent (Docker)              | No            | No             | `~/.openhands`                      |
| Zed AI          | `zed-ai`         | Yes          | Yes | Full (editor)                    | No            | No             | `~/.config/zed`                     |
| GitHub Copilot  | `github-copilot` | Yes          | Yes | Full (VS Code ext)               | No            | No             | `~/.config/github-copilot`          |
| Devin           | `devin`          | Yes          | Yes | Solo-agent (web)                 | No            | No             | `~/.devin`                          |
| Cody            | `cody`           | Yes          | Yes | Full (VS Code ext)               | No            | No             | `~/.sourcegraph`                    |
| Tabnine         | `tabnine`        | Yes          | Yes | Solo-agent (VS Code ext)         | No            | No             | `~/.tabnine`                        |
| Replit          | `replit`         | Yes          | Yes | Solo-agent (browser)             | No            | No             | `~/.replit`                         |
| Void            | `void`           | Yes          | Yes | Full (VS Code fork)              | No            | No             | `~/.void`                           |

Most agents receive the **full SDD orchestrator** policy, plus skill files written to their skills directory. Most receive it through their system prompt; OpenCode and Kilo Code receive it through the OpenCode-compatible `opencode.json` agent overlay. Pi is the exception: Dxrk AI installs Pi packages, and `dxrk-pi` owns Pi skills, prompts, SDD agents, and chains at runtime. The agent handles SDD automatically when the task is large enough, or when the user explicitly asks for it — no manual setup required.

---

## Delegation Models

| Model                 | How It Works                                                                                                                                                                                       | Agents                                                                                                    |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **Full (sub-agents)** | Each SDD phase runs in an isolated context window via native sub-agent delegation, package-managed subagents, or an OpenCode-compatible overlay. The orchestrator coordinates; sub-agents execute. | Claude Code, OpenCode, Kilo Code, Gemini CLI, Cursor, VS Code Copilot, Kimi Code, Kiro IDE, Qwen Code, Pi, Cline, Roo Code, Continue, Junie, Amazon Q, Zed AI, GitHub Copilot, Cody, Void |
| **Solo-agent**        | All SDD phases run inline in the same conversation. The orchestrator IS the executor. Dxrk-Memory provides cross-phase persistence.                                                                     | Codex, Windsurf, Antigravity, OpenClaw, Aider, OpenHands, Devin, Tabnine, Replit |

### Cursor Native Subagents

Cursor uses its built-in `.cursor/agents/` system. `dxrk` writes 10 agent files to `~/.cursor/agents/sdd-{phase}.md` — one per SDD phase. Cursor's Agent auto-delegates to the correct subagent based on the `description` field in each file's YAML frontmatter.

- `sdd-explore` and `sdd-verify` run with `readonly: false` so they can inspect the codebase and execute verification commands
- Each subagent gets its own context window (fresh context, no pollution)
- The orchestrator resolves compact rules from the skill registry and passes them in the invocation message

### Windsurf Cascade

Windsurf runs as a solo-agent (no custom sub-agents). The orchestrator leverages Windsurf-native features:

- **Plan Mode** — creates persistent plan documents that can be @mentioned across sessions; ideal for spec and design artifacts on large changes
- **Code Mode** — default agentic execution mode
- **Native Workflows** — `sdd-new` is available as a `.windsurf/workflows/sdd-new.md` workflow
- **Size Classification** — the orchestrator routes tasks through Small/Medium/Large decision paths

### Antigravity + Mission Control

Antigravity is an agent-first platform with built-in sub-agents (Browser, Terminal) managed by Mission Control. However, custom sub-agent creation is not yet available. SDD phases run inline, with Mission Control handling automatic delegation to built-in sub-agents when specialized tooling is needed (e.g., Browser for research during `sdd-explore`).

### Kiro Native Subagents

Kiro uses native custom agents in `~/.kiro/agents/`. `dxrk` writes 10 phase agents (`sdd-init` through `sdd-onboard`) and resolves the `model:` field during injection from Claude alias assignments (`opus|sonnet|haiku`) to Kiro-native model IDs.

- Frontmatter includes `includeMcpJson: true` for all phase agents
- Phase-specific tools are preserved (`sdd-explore` and `sdd-verify` use read/shell/context7 as required)
- Orchestrator remains in steering (`~/.kiro/steering/dxrk.md`) and delegates execution to native subagents

---

## SDD Mode Support

| Feature          | Claude Code | OpenCode | Kilo Code | Gemini CLI | Cursor | VS Code Copilot | Codex | Windsurf | Antigravity | Kiro IDE | Qwen Code | OpenClaw |   Pi    |
| ---------------- | :---------: | :------: | :-------: | :--------: | :----: | :-------------: | :---: | :------: | :---------: | :------: | :-------: | :------: | :-----: |
| SDD orchestrator |     Yes     |   Yes    |    Yes    |    Yes     |  Yes   |       Yes       |  Yes  |   Yes    |     Yes     |   Yes    |    Yes    |   Yes    |   Yes   |
| Single-mode SDD  |     Yes     |   Yes    |    Yes    |    Yes     |  Yes   |       Yes       |  Yes  |   Yes    |     Yes     |   Yes    |    Yes    |   Yes    |   Yes   |
| Multi-mode SDD   |      —      |   Yes    |    Yes    |     —      |   —    |        —        |   —   |    —     |      —      |  Yes\*   |     —     |    —     | Yes\*\* |

**Multi-mode** (assigning different AI models to each SDD phase) is supported by **OpenCode** and **Kilo Code** through the OpenCode-compatible multi-mode overlay, and by **Kiro IDE** through native subagent `model:` frontmatter. All other agents run in **single-mode** — the orchestrator manages everything using whatever model the agent is already running.

> \* **Kiro multi-mode** assigns models per phase through `KiroModelAssignments` (configured via _Configure Models → Configure Kiro models_ in the TUI). The selected alias (`opus|sonnet|haiku`) is resolved to a Kiro-native model ID and stamped into each `~/.kiro/agents/sdd-{phase}.md` at sync time.

> \*\* **Pi multi-mode** is owned by the Pi packages. `dxrk-pi` installs SDD agent and chain assets into `.pi/agents/` and `.pi/chains/`; model overrides live in those Pi-managed files or chain steps.

---

## Agent Notes

### Claude Code

- Sub-agents via the native Task tool with isolated context windows
- MCP servers configured as plugins in `~/.claude/mcp/`
- Output styles in `~/.claude/output-styles/`
- System prompt via markdown sections in `~/.claude/CLAUDE.md`

### OpenCode

- Full multi-agent overlay with 11 named agents in `opencode.json` (`dxrk-orchestrator` plus 10 SDD phase agents)
- Slash commands for SDD phases (`/sdd-new`, `/sdd-explore`, etc.)
- Background-agents plugin for parallel execution
- The TUI model picker includes providers and models discovered from the local `opencode.json`, including custom providers
- Custom models from `opencode.json` must set `tool_call: true` explicitly to appear as selectable SDD-capable options in the model picker
- Multi-mode prerequisite: connect your AI providers first, then run `opencode models --refresh`

### Kilo Code

- **Detection**: dxrk detects Kilo Code from `~/.config/kilo` and checks for the `kilo` binary on `PATH`
- Uses the OpenCode-compatible adapter: `AGENTS.md`, `skills/`, `commands/`, and `opencode.json` live under `~/.config/kilo`
- Full SDD delegation is provided by the merged multi-agent overlay in `~/.config/kilo/opencode.json`, not by a separate native sub-agent directory
- MCP servers are merged into `opencode.json`; Dxrk-Memory uses the OpenCode-style local MCP entry with `command` as an array
- Auto-install is supported via npm: `npm install -g @kilocode/cli`

### Gemini CLI

- Sub-agents are experimental: require `experimental.enableAgents: true` in `settings.json`
- Custom sub-agents defined as markdown files in `~/.gemini/agents/`

### Cursor

- Native subagents via `~/.cursor/agents/sdd-{phase}.md` (10 files installed by dxrk)
- Skills at `~/.cursor/skills/`
- System prompt in `~/.cursor/rules/dxrk.mdc`
- MCP config in `~/.cursor/mcp.json`

### VS Code Copilot

- Uses the `runSubagent` tool with support for parallel execution
- Skills at `~/.copilot/skills/`
- System prompt at `Code/User/prompts/dxrk.instructions.md`
- MCP config at `Code/User/mcp.json`

### Codex

- CLI-native agent with TOML config at `~/.codex/config.toml`
- Skills at `~/.codex/skills/`
- System prompt at `~/.codex/agents.md`
- Dxrk-Memory instruction files at `~/.codex/dxrk-memory-instructions.md`

### Windsurf

- Skills at `~/.codeium/windsurf/skills/` (native Windsurf feature)
- MCP config at `~/.codeium/windsurf/mcp_config.json`
- Global rules at `~/.codeium/windsurf/memories/global_rules.md`
- Workflows at `.windsurf/workflows/` (workspace-scoped)

### Antigravity

- Skills at `~/.gemini/antigravity/skills/` (native Antigravity feature)
- MCP config at `~/.gemini/antigravity/mcp_config.json`
- System prompt appended to `~/.gemini/GEMINI.md` (shared with Gemini CLI — collision check warns if both are installed)
- Mission Control handles built-in sub-agent delegation (Browser, Terminal) automatically
- Settings managed via the IDE's Agent settings UI, not via `settings.json`

### Kimi Code

- Installation requires the `uv` Python package manager (`uv tool install kimi-cli`).
- Root custom agent at `~/.kimi/agents/dxrk.yaml` with `system_prompt_path: ../KIMI.md`
- `KIMI.md` is a thin Jinja template that includes modular prompt files:
  `persona.md`, `output-style.md`, `dxrk-memory-protocol.md`, `sdd-orchestrator.md`
- Built-in Kimi variables are preserved in `KIMI.md`: `${KIMI_AGENTS_MD}` and `${KIMI_SKILLS}`

### Kiro IDE

- **Detection**: dxrk detects Kiro from the `kiro` binary on `PATH`; when the binary is present, it also reports whether `~/.kiro` already exists. A config directory alone does not mark Kiro as installed.
- **Steering file** (all platforms): `~/.kiro/steering/dxrk.md` with frontmatter `inclusion: always`
- Native subagents at `~/.kiro/agents/sdd-{phase}.md` (10 files)
- Skills (all platforms) at `~/.kiro/skills/`
- **MCP config at a separate root** — always `~/.kiro/settings/mcp.json` (macOS/Linux) or `%USERPROFILE%\.kiro\settings\mcp.json` (Windows), regardless of GlobalConfigDir
- Native Kiro specs workflow: `.kiro/specs/<feature>/requirements.md`, `design.md`, `tasks.md` — with approval gates before apply and archive phases
- Manual install only — download from [kiro.dev/downloads](https://kiro.dev/downloads)
- See [docs/kiro.md](kiro.md) for full path reference and SDD behavior details

### Qwen Code

- **Detection**: dxrk detects Qwen Code from its config root (`~/.qwen`) and checks for `qwen` binary on `PATH`
- **Config root**: `~/.qwen/` (cross-platform)
- **System prompt**: `~/.qwen/QWEN.md` (managed via `StrategyFileReplace`)
- **Skills**: `~/.qwen/skills/`
- **MCP config**: `~/.qwen/settings.json` (managed via `StrategyMergeIntoSettings` with `mcpServers` key)
- **Slash commands**: `~/.qwen/commands/*.md` — supports custom namespaced slash commands (e.g., `commands/sdd/init.md` → `/sdd:init`)
- **Permissions**: `auto_edit` mode — auto-approves file edits, manual approval for shell commands
- **Install**: via npm — `npm install -g @qwen-code/qwen-code@latest`
- **Dxrk-Memory slug**: `"qwen-code"` for `dxrk-memory setup` integration
- **SDD orchestrator**: `internal/assets/qwen/sdd-orchestrator.md` with Qwen-specific path references

### OpenClaw

- **Detection**: dxrk detects OpenClaw from the `openclaw` binary on `PATH` and its config root at `~/.openclaw`.
- **Install**: manual only — install OpenClaw first, then run `dxrk install --agent openclaw`.
- **Active workspace**: dxrk reads `agents.defaults.workspace` from `~/.openclaw/openclaw.json` and writes instruction files there.
- **Instructions**: Dxrk-Memory and SDD protocols are injected into workspace `AGENTS.md`; persona is injected into workspace `SOUL.md`.
- **MCP config**: Dxrk-Memory and Context7 are merged into global `~/.openclaw/openclaw.json` under `mcp.servers`; legacy root `mcpServers` entries are migrated.
- **Skills**: SDD phase skills are workspace-scoped at `<workspace>/.openclaw/skills/sdd-*`; portable skills remain global at `~/.openclaw/skills/`.

### Aider

- **Detection**: dxrk detects Aider from the `aider` binary on `PATH` (installed via `pip install aider-chat`).
- **Install**: `pip install aider-chat` (auto-installable).
- **Config**: `~/.aider.conf.yml` (YAML configuration).
- **System prompt**: Appended to `~/.aider.conf.yml` via `StrategyAppendToFile`.
- **MCP config**: Merged into settings via `StrategyMergeIntoSettings`.
- **Note**: Aider is a git-aware pair programmer that auto-commits changes.

### Cline

- **Detection**: dxrk detects Cline from the VS Code extension directory.
- **Install**: manual — install Cline extension from VS Code marketplace.
- **Config**: `~/.cline/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written to `~/.cline/mcp_config.json` via `StrategyMCPConfigFile`.
- **Note**: Autonomous multi-step coding agent with terminal and browser automation.

### Roo Code

- **Detection**: dxrk detects Roo Code from the VS Code extension directory.
- **Install**: manual — install Roo Code extension from VS Code marketplace.
- **Config**: `~/.roo/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written to `~/.roo/mcp_config.json` via `StrategyMCPConfigFile`.
- **Note**: Cline fork with multi-mode agents (Code, Architect, Ask, Debug).

### Continue

- **Detection**: dxrk detects Continue from the IDE extension directory.
- **Install**: manual — install Continue extension (VS Code, JetBrains, or Neovim).
- **Config**: `~/.continue/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written to `~/.continue/mcp_config.json` via `StrategyMCPConfigFile`.
- **Note**: Multi-IDE open-source AI assistant with BYO model support.

### Junie

- **Detection**: dxrk detects Junie from the JetBrains plugin directory.
- **Install**: manual — install Junie plugin from JetBrains Marketplace.
- **Config**: `~/.junie/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Merged into settings via `StrategyMergeIntoSettings`.
- **Note**: JetBrains-native AI agent with debugger and semantic index access.

### Amazon Q

- **Detection**: dxrk detects Amazon Q from the VS Code extension directory.
- **Install**: manual — install Amazon Q extension from VS Code marketplace.
- **Config**: `~/.aws/q/`.
- **System prompt**: Appended via `StrategyAppendToFile`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: Deep AWS integration with security scanning and .NET migration support.

### OpenHands

- **Detection**: dxrk detects OpenHands from the `openhands` binary on `PATH` (installed via `pip install openhands-ai`).
- **Install**: `pip install openhands-ai` (auto-installable).
- **Config**: `~/.openhands/`.
- **System prompt**: Appended via `StrategyAppendToFile`.
- **MCP config**: Merged into settings via `StrategyMergeIntoSettings`.
- **Note**: Docker-based autonomous coding agent with browser and terminal sandbox.

### Zed AI

- **Detection**: dxrk detects Zed from the `zed` binary on `PATH`.
- **Install**: manual — download from [zed.dev](https://zed.dev).
- **Config**: `~/.config/zed/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: High-performance Rust editor with multi-provider LLM support.

### GitHub Copilot

- **Detection**: dxrk detects Copilot from the VS Code extension directory.
- **Install**: manual — install GitHub Copilot extension from VS Code marketplace.
- **Config**: `~/.config/github-copilot/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: ~20M users, agent mode, PR reviews, broadest IDE support.

### Devin

- **Detection**: dxrk detects Devin from the `~/.devin` config directory.
- **Install**: manual — sign up at [devin.ai](https://devin.ai).
- **Config**: `~/.devin/`.
- **System prompt**: Appended via `StrategyAppendToFile`.
- **MCP config**: Merged into settings via `StrategyMergeIntoSettings`.
- **Note**: Fully autonomous software engineer with own IDE, browser, and terminal.

### Cody

- **Detection**: dxrk detects Cody from the VS Code extension directory.
- **Install**: manual — install Sourcegraph Cody extension from VS Code marketplace.
- **Config**: `~/.sourcegraph/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: Code graph for large codebase context, enterprise focus.

### Tabnine

- **Detection**: dxrk detects Tabnine from the VS Code extension directory.
- **Install**: manual — install Tabnine extension from VS Code marketplace.
- **Config**: `~/.tabnine/`.
- **System prompt**: Appended via `StrategyAppendToFile`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: Privacy-first code completion with enterprise self-hosting.

### Replit

- **Detection**: dxrk detects Replit from the `~/.replit` config directory.
- **Install**: manual — sign up at [replit.com](https://replit.com).
- **Config**: `~/.replit/`.
- **System prompt**: Appended via `StrategyAppendToFile`.
- **MCP config**: Merged into settings via `StrategyMergeIntoSettings`.
- **Note**: Browser-based full-stack app builder from prompts.

### Void

- **Detection**: dxrk detects Void from the editor directory.
- **Install**: manual — download from [voideditor.com](https://voideditor.com).
- **Config**: `~/.void/`.
- **System prompt**: Markdown sections via `StrategyMarkdownSections`.
- **MCP config**: Written via `StrategyMCPConfigFile`.
- **Note**: Open-source AI editor (VS Code fork) with BYO models.

### Pi

For the full Pi command and package reference, see [Pi Agent](pi.md).

- **Detection**: dxrk detects Pi from the `pi` binary on `PATH` and its config root at `~/.pi`.
- **Install**: Pi must already be installed. dxrk then installs the full Pi support stack with:
  - `pi install npm:dxrk-pi`
  - `pi install npm:dxrk-dxrk-memory`
  - `pi install npm:pi-mcp-adapter`
  - `npm exec --yes --package dxrk-dxrk-memory@0.1.4 -- pi-dxrk-memory init`
  - `pi install npm:pi-subagents`
  - `pi install npm:pi-intercom`
  - `pi install npm:@juicesharp/rpiv-ask-user-question`
  - `pi install npm:pi-web-access`
  - `pi install npm:pi-lens`
  - `pi install npm:@juicesharp/rpiv-todo`
  - `pi install npm:pi-btw`
- **`dxrk-pi` package**: adds the Dxrk harness for Pi: SDD/OpenSpec workflow, strict TDD guidance, safety defaults, `/dxrk:*` commands, skill assets, prompts, SDD agents, and SDD chains. On normal `session_start`, it copies project assets into `.pi/agents/`, `.pi/chains/`, and `.pi/dxrk/support/` without overwriting local files unless the Pi recovery command uses `--force`. Starting Pi with `pi -ns` skips startup skill loading/hooks, so that automatic refresh does not run in that mode.
- **Package metadata**: latest verified `dxrk-pi` version is `0.2.6`; npm lists `alan_buscaglia` as maintainer, with source at [Dxrk777/dxrk-pi](https://github.com/Dxrk777/dxrk-pi) and package docs at [npm: dxrk-pi](https://www.npmjs.com/package/dxrk-pi).
- **Persona command**: `dxrk-pi` owns Pi persona switching through `/dxrk:persona` (`/dxrk:persona` remains a compatibility alias). It switches between `dxrk` and `neutral`, saves `.pi/dxrk/persona.json`, and may require `/reload` or a new Pi session for the active prompt to refresh.
- **Model assignment command**: `dxrk-pi` owns Pi model selection through `/dxrk:models` (`/dxrk:models` remains a compatibility alias). It opens a Pi-native modal for project, user, and built-in agents, prioritizes SDD agents, saves `.pi/dxrk/models.json`, and applies overrides into `.pi/agents/*.md` or `.pi/settings.json`.
- **`dxrk-dxrk-memory` package**: adds persistent Dxrk-Memory memory for Pi. It captures sessions, exposes Dxrk-Memory MCP tools through `pi-mcp-adapter`, and degrades safely when the local `dxrk-memory` binary is missing.
- **MCP adapter wiring**: ComponentDxrk-Memory declares `npm:pi-mcp-adapter` in `.pi/agent/settings.json` packages and adds `pi-mcp-adapter` `^2.6.0` to `.pi/npm/package.json` without removing unrelated user entries. `pi-dxrk-memory init` owns the Pi Dxrk-Memory MCP config schema and is run during installation.
- **`pi-subagents` package**: discovers and runs SDD agents from `.pi/agents/`.
- **`pi-intercom` package**: lets Pi child agents ask the parent session for decisions while a chain is running.
- **`@juicesharp/rpiv-ask-user-question` package**: lets Pi child agents ask the active user session for clarification when they need human input.
- **Pi companion packages**: `pi-web-access`, `pi-lens`, `@juicesharp/rpiv-todo`, and `pi-btw` add web access, context inspection, todo tracking, and companion workflow support.
- **Pi-only flow**: when Pi is the only selected agent, dxrk skips persona, ecosystem component selection, and Strict TDD prompts because those behaviors are provided by `dxrk-pi`.
