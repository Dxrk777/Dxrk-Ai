# Pi Agent

← [Back to README](../README.md)

Pi support installs the Dxrk harness as Pi packages, then lets Pi own its own persona, models, SDD agents, chains, and memory wiring.

## Quick Start

1. Install Pi and make sure `pi` is available on `PATH`.
2. Install the Pi support stack from Dxrk AI:

```bash
dxrk install --agent pi
```

3. Start Pi in your project:

```bash
pi
```

Dxrk AI detects the `pi` binary first. If Pi is the only selected agent, the installer still provisions the real Dxrk-Memory component, but skips persona, ecosystem component selection, and Strict TDD prompts because `dxrk-pi` owns those choices inside Pi.

## Installed Packages

Dxrk AI runs exactly these Pi setup steps:

```bash
pi install npm:dxrk-pi
pi install npm:dxrk-dxrk-memory
pi install npm:pi-mcp-adapter
npm exec --yes --package dxrk-dxrk-memory@0.1.4 -- pi-dxrk-memory init
pi install npm:pi-subagents
pi install npm:pi-intercom
pi install npm:@juicesharp/rpiv-ask-user-question
pi install npm:pi-web-access
pi install npm:pi-lens
pi install npm:@juicesharp/rpiv-todo
pi install npm:pi-btw
```

| Package                                                  | What it adds                                                                                                              |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| [`dxrk-pi`](https://www.npmjs.com/package/dxrk-pi)   | Dxrk persona, SDD/OpenSpec workflow, strict TDD support, safety policy, skills, prompts, SDD agents, and SDD chains. |
| [`dxrk-dxrk-memory`](https://pi.dev/packages/dxrk-dxrk-memory) | Pi integration for Dxrk-Memory session memory and MCP tools. It is not the Dxrk-Memory binary itself.                               |
| `pi-mcp-adapter`                                         | Lets Pi expose MCP servers, including Dxrk-Memory, through Pi's MCP runtime.                                                   |
| `pi-dxrk-memory init`                                         | Initializes the Pi Dxrk-Memory MCP config shape owned by `dxrk-dxrk-memory`.                                                      |
| `pi-subagents`                                           | Runs SDD agents discovered from `.pi/agents/`.                                                                            |
| `pi-intercom`                                            | Lets child agents ask the parent Pi session for decisions while chains run.                                               |
| `@juicesharp/rpiv-ask-user-question`                     | Lets Pi child agents ask the active user session for clarification when they need human input.                            |
| `pi-web-access`                                          | Adds web access tools for Pi.                                                                                             |
| `pi-lens`                                                | Adds Pi visual/context inspection support.                                                                                |
| `@juicesharp/rpiv-todo`                                  | Adds todo/task tracking support for Pi sessions.                                                                          |
| `pi-btw`                                                 | Adds BTW companion workflow support for Pi.                                                                               |

`dxrk-pi` owns Pi's runtime behavior. Its current harness enforces parent-only delegation triggers: delegate exploration after 4+ files, use one writer for multi-file changes, require fresh review before PRs, run fresh audits after incidents, and pause long monolithic sessions before they drift.

The real Dxrk-Memory component is provisioned separately by Dxrk AI so `dxrk-dxrk-memory` has an Dxrk-Memory runtime to talk to.
During that Dxrk-Memory provisioning step, Dxrk AI declares `npm:pi-mcp-adapter` in Pi's agent settings and adds the npm dependency. Existing unrelated Pi settings, package entries, and npm dependencies are preserved.

Files updated by Dxrk AI's Dxrk-Memory provisioning:

```text
.pi/agent/settings.json    # packages includes npm:pi-mcp-adapter
.pi/npm/package.json       # dependencies.pi-mcp-adapter = ^2.6.0
```

`dxrk-dxrk-memory` owns the MCP schema itself. The installer runs `pi-dxrk-memory init`, which initializes Pi's Dxrk-Memory MCP config under the Pi agent config directory instead of having Dxrk AI hand-write that file.

## Pi Commands

Run these inside Pi after installing the package stack.

| Command                          | What it does                                                                                                    |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `/dxrk:status`              | Shows package, SDD asset, OpenSpec, and model config status.                                                    |
| `/dxrk:persona`             | Switches between `dxrk` and `neutral` personas.                                                            |
| `/dxrk:persona`             | Compatibility alias for `/dxrk:persona`.                                                                   |
| `/dxrk:models`              | Opens the Pi-native model assignment modal.                                                                     |
| `/dxrk:models`              | Compatibility alias for `/dxrk:models`.                                                                    |
| `/sdd-init`                      | Bootstraps or refreshes `openspec/config.yaml`.                                                                 |
| `/dxrk:install-sdd`         | Reinstalls SDD assets without overwriting local files.                                                          |
| `/dxrk:install-sdd --force` | Force-refreshes installed SDD assets. Use this when you explicitly want package assets to replace local copies. |

## Persona Selection

Pi persona selection belongs to `dxrk-pi`, not the Dxrk AI installer.

```text
/dxrk:persona
```

| Persona     | Behavior                                                                                                                   |
| ----------- | -------------------------------------------------------------------------------------------------------------------------- |
| `dxrk` | Teaching-oriented senior architect persona with Rioplatense Spanish/voseo when the user writes Spanish.                    |
| `neutral`   | Same senior architect discipline and teaching philosophy, but with warm professional language and no regional expressions. |

The selection is saved at:

```text
.pi/dxrk/persona.json
```

Run `/reload` or start a new Pi session after switching if the current session already injected the previous persona.

## Model Assignments

Pi model assignment belongs to `dxrk-pi`, not the Dxrk AI installer.

```text
/dxrk:models
```

The modal discovers project, user, and built-in agents. SDD agents are shown first so you can tune the phases that matter most.

| Agent kind                     | Recommended model shape                                              |
| ------------------------------ | -------------------------------------------------------------------- |
| Exploration, proposal, archive | Fast and cheap is usually enough.                                    |
| Spec, design, tasks            | Strong reasoning model, because these phases shape implementation.   |
| Apply                          | Strong coding model with reliable tool use.                          |
| Verify / review agents         | Strong fresh-context model. Verification benefits from independence. |
| Tiny utility agents            | Inherit the active/default model unless they become a bottleneck.    |

Saved config:

```text
.pi/dxrk/models.json
```

Applied configuration:

```text
.pi/agents/*.md
.pi/settings.json
```

Use `Inherit active/default model` to remove an agent override.

## Project Files

On normal Pi `session_start`, `dxrk-pi` copies project-local assets without overwriting local edits:

```text
.pi/agents/sdd-*.md
.pi/chains/sdd-*.chain.md
.pi/dxrk/support/strict-tdd.md
.pi/dxrk/support/strict-tdd-verify.md
```

Use `/dxrk:install-sdd --force` only when you want to replace local SDD assets with the package version.

If you start Pi with `pi -ns`, Pi skips startup skill loading/hooks. That mode is useful for a clean or faster Pi session, but it also means `dxrk-pi` startup work such as asset checks and skill-registry refreshes will not run automatically.

## Troubleshooting

| Symptom                                                | Fix                                                                                                                                                                  |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Dxrk AI says Pi is missing                           | Install Pi first and make sure `pi` is on `PATH`.                                                                                                                    |
| SDD agents are missing in Pi                           | Start Pi normally in the project so `dxrk-pi` can run `session_start`, or run `/dxrk:install-sdd`. If you used `pi -ns`, startup hooks were skipped.          |
| Persona did not change immediately                     | Run `/reload` or start a new Pi session.                                                                                                                             |
| Model override should be removed                       | Open `/dxrk:models` and choose `Inherit active/default model`.                                                                                                  |
| Memory tools or `/mcp` are missing                     | Re-run `dxrk install --agent pi` to refresh `.pi/agent/settings.json`, `.pi/npm/package.json`, and the `pi-dxrk-memory init` wiring, then check `/dxrk:status`. |
| `dxrk-dxrk-memory` is installed but Dxrk-Memory is unavailable | Re-run `dxrk install --agent pi` so the real Dxrk-Memory component is provisioned.                                                                                   |

## Next Steps

- Read [Supported Agents](agents.md) for the full agent matrix.
- Read [Dxrk-Memory Commands](dxrk-memory.md) if you want to inspect or sync persistent memory.
- Read [Usage](usage.md) for the general Dxrk AI CLI and TUI flow.

← [Back to README](../README.md)
