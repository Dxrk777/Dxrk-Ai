# Dxrk-Memory Command Reference

<- [Back to README](../README.md)

---

Dxrk-Memory works automatically. Your AI agent saves decisions, discoveries, and context to persistent memory without you doing anything. You do not need to memorize commands or manage memory manually.

This page exists for when you want to inspect, share, or fix your memories by hand.

---

## Day-to-Day Commands

These are the only commands most people ever need.

```bash
# Browse memories visually -- search, filter, drill into observations
dxrk-memory tui

# Search from the terminal without opening the TUI
dxrk-memory search "auth refactor"

# Export project memories to .dxrk-memory/ so you can commit them to git
dxrk-memory sync
```

`dxrk-memory tui` is the fastest way to see what your agent has been saving. Start there.

---

## Project Management

Dxrk-Memory groups memories by project name, auto-detected from your git remote since v1.11.0. Sometimes projects end up with duplicate names (e.g., "my-app" vs "My-App" vs "my-app-frontend"). These commands fix that.

```bash
# List all projects with observation counts
dxrk-memory projects list

# Interactively merge duplicate project names into one
dxrk-memory projects consolidate
```

`projects list` shows every project dxrk-memory knows about and how many observations each has. If you see the same project under multiple names, run `projects consolidate` to merge them.

The MCP equivalent is `mem_merge_projects`, which the AI agent can call directly when it detects name drift.

---

## Team Sharing

Dxrk-Memory memories live locally by default. To share them with your team via git:

```bash
# After a work session -- export memories to .dxrk-memory/ in your repo
dxrk-memory sync

# On another machine -- import memories after cloning
dxrk-memory sync --import
```

Add `.dxrk-memory/` to your repo and commit it. When a teammate clones and runs `dxrk-memory sync --import`, they get the full project context. This is especially useful for onboarding -- new contributors start with the accumulated knowledge of the team.

---

## MCP Tools Reference

These are the tools the AI agent uses behind the scenes. You never call them directly, but understanding them helps you know what your agent is doing.

### Core Tools

| Tool | What it does |
|------|--------------|
| `mem_save` | Saves a decision, bug fix, discovery, or convention to memory. Dxrk-Memory v1.15.3+ captures the user prompt best-effort by default when prompt context was already fed for the same project/session |
| `mem_search` | Searches memory by keywords -- returns matching observations |
| `mem_context` | Gets recent session history (called at session start) |
| `mem_session_summary` | Saves an end-of-session summary so the next session has context |
| `mem_get_observation` | Retrieves full untruncated content of a specific observation by ID |
| `mem_save_prompt` | Saves the user's prompt and feeds session activity so a later `mem_save` can capture/dedupe it |

`mem_save` accepts optional `capture_prompt`. Leave it unset for normal human/proactive saves. Use `capture_prompt: false` only for automated artifacts such as SDD proposal/spec/design/tasks/apply/verify/archive/init reports, testing-capabilities caches, onboarding/state artifacts, or skill-registry output. If the MCP server has no prompt context, `mem_save` still succeeds and does not invent prompt text.

Agents or plugin hooks that can observe the user's prompt should call `mem_save_prompt` before any derived `mem_save` calls so Dxrk-Memory can attach and dedupe the real prompt context.

### Advanced Tools

<details>
<summary>Click to expand -- rarely needed, but available</summary>

| Tool | What it does |
|------|--------------|
| `mem_update` | Updates an existing observation by ID |
| `mem_suggest_topic_key` | Suggests a stable topic key for evolving topics |
| `mem_session_start` / `mem_session_end` | Session lifecycle management |
| `mem_stats` | Memory statistics (observation count, project breakdown) |
| `mem_delete` | Deletes an observation by ID |
| `mem_timeline` | Chronological view of observations |
| `mem_capture_passive` | Extracts learnings from conversation passively |
| `mem_merge_projects` | Merges project name variants (CLI equivalent: `dxrk-memory projects consolidate`) |

</details>

---

## How Project Detection Works

Since v1.11.0, dxrk-memory reads the git remote URL at startup, normalizes it to lowercase, and uses that as the project name. If it finds similar existing project names, it warns you. This prevents the most common issue -- the same project accumulating memories under slightly different names.

If you're working outside a git repo, dxrk-memory falls back to the directory name.

---

## Full Documentation

For the complete source, configuration options, and contribution guide: [github.com/Dxrk777/dxrk-memory](https://github.com/Dxrk777/dxrk-memory)
