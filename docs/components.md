# Components, Skills & Presets

← [Back to README](../README.md)

---

## Components

| Component | ID | Description |
|-----------|-----|-------------|
| Dxrk-Memory | `dxrk-memory` | Persistent cross-session memory via MCP — auto-detection of project name, full-text search, git sync, project consolidation. See [dxrk-memory repo](https://github.com/Dxrk777/dxrk-memory) |
| SDD | `sdd` | Spec-Driven Development workflow (9 phases) — the agent handles SDD organically when the task warrants it, or when you ask; you don't need to learn the commands |
| Skills | `skills` | Curated coding skill library |
| Context7 | `context7` | MCP server for live framework/library documentation |
| Persona | `persona` | Managed Dxrk/neutral persona injection, or unmanaged custom persona mode |
| Permissions | `permissions` | Security-first defaults and guardrails |
| Dxrk-Guardian | `dxrk-guardian` | Dxrk Guardian Angel — AI provider switcher |
| Theme | `theme` | Dxrk Kanagawa theme overlay |

## Dxrk-Guardian Behavior

`dxrk --component dxrk-guardian` installs/provisions the `dxrk-guardian` binary globally on your machine.

It does **not** run project-level hook setup automatically (`dxrk-guardian init` / `dxrk-guardian install`) because that should be an explicit decision per repository.

After global install, enable Dxrk-Guardian per project with:

```bash
dxrk-guardian init
dxrk-guardian install
```

---

## Skills

### Included Skills (installed by dxrk)

20 skill files organized by category, embedded in the binary and injected into your agent's configuration:

#### SDD (Spec-Driven Development)

| Skill | ID | Description |
|-------|-----|-------------|
| SDD Init | `sdd-init` | Bootstrap SDD context in a project |
| SDD Explore | `sdd-explore` | Investigate codebase before committing to a change |
| SDD Propose | `sdd-propose` | Create change proposal with intent, scope, approach |
| SDD Spec | `sdd-spec` | Write specifications with requirements and scenarios |
| SDD Design | `sdd-design` | Technical design with architecture decisions |
| SDD Tasks | `sdd-tasks` | Break down a change into implementation tasks |
| SDD Apply | `sdd-apply` | Implement tasks following specs and design |
| SDD Verify | `sdd-verify` | Validate implementation matches specs |
| SDD Archive | `sdd-archive` | Sync delta specs to main specs and archive |
| SDD Onboard | `sdd-onboard` | Guided end-to-end SDD walkthrough on the real codebase |
| Judgment Day | `judgment-day` | Parallel adversarial review — two independent judges review the same target |

#### Foundation

| Skill | ID | Description |
|-------|-----|-------------|
| Go Testing | `go-testing` | Go testing patterns including Bubbletea TUI testing |
| Skill Creator | `skill-creator` | Create new AI agent skills following the Agent Skills spec |
| Branch & PR | `branch-pr` | PR creation workflow with conventional commits, branch naming, and issue-first enforcement |
| Issue Creation | `issue-creation` | Issue filing workflow with bug report and feature request templates |
| Skill Registry | `skill-registry` | Build a compact project standards registry from installed skills |
| Chained PR | `chained-pr` | Plan and create reviewable stacked/chained pull requests |
| Cognitive Doc Design | `cognitive-doc-design` | Write docs that reduce review and onboarding cognitive load |
| Comment Writer | `comment-writer` | Draft warm, direct collaboration comments and review replies |
| Work Unit Commits | `work-unit-commits` | Split implementation into reviewable work units |

These foundation skills are installed by default with both `full-dxrk` and `ecosystem-only` presets.

### Coding Skills (separate repository)

For framework-specific skills (React 19, Angular, TypeScript, Tailwind 4, Zod 4, Playwright, etc.), see [Dxrk777/Dxrk-Skills](https://github.com/Dxrk777/Dxrk-Skills). These are maintained by the community and installed separately by cloning the repo and copying skills to your agent's skills directory.

---

## Presets

| Preset | ID | What's Included |
|--------|-----|-----------------|
| Full Dxrk | `full-dxrk` | All components (Dxrk-Memory + SDD + Skills + Context7 + Dxrk-Guardian + Persona + Permissions + Theme) + all skills + dxrk persona |
| Ecosystem Only | `ecosystem-only` | Core components (Dxrk-Memory + SDD + Skills + Context7 + Dxrk-Guardian) + all skills + dxrk persona |
| Minimal | `minimal` | Dxrk-Memory + SDD skills only |
| Custom | `custom` | You choose components and skills manually while keeping any existing persona/settings unmanaged |
