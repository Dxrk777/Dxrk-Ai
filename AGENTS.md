# Dxrk — Agent Skills Index

**Total: 2,242 skills** (34 dxrk + 2,208 imported from 4 community collections)

When working on this project, load the relevant skill(s) BEFORE writing any code.

## Imported Collections
- `anthropics/skills` — +16 official
- `sickn33/antigravity-awesome-skills` — +1,879 (the 1,370+ collection)
- `ComposioHQ/awesome-claude-skills` — +18 new
- `alirezarezvani/claude-skills` — +293 new

Naming convention: `dxrk-*` skills are repo-specific workflow skills. Unprefixed skills are portable writing or work-unit skills and intentionally keep their canonical names.

## How to Use

1. Check the trigger column to find skills that match your current task
2. Load the skill by reading the SKILL.md file at the listed path
3. Follow ALL patterns and rules from the loaded skill
4. Multiple skills can apply simultaneously

## Skills

| Skill | Trigger | Path |
|-------|---------|------|
| `dxrk-issue-creation` | When creating a GitHub issue, reporting a bug, or requesting a feature. | [`skills/issue-creation/SKILL.md`](skills/issue-creation/SKILL.md) |
| `dxrk-branch-pr` | When creating a pull request, opening a PR, or preparing changes for review. | [`skills/branch-pr/SKILL.md`](skills/branch-pr/SKILL.md) |
| `dxrk-testing` | When running, debugging, or choosing which tests to run. | `.agents/skills/dxrk-testing/SKILL.md` |
| `dxrk-test-performance` | When debugging slow tests, measuring test time, or optimizing test performance. | `.agents/skills/dxrk-test-performance/SKILL.md` |
| `dxrk-test-memory` | When investigating OOMs, heap leaks, or memory growth in tests. | `.agents/skills/dxrk-test-memory/SKILL.md` |
| `dxrk-test-optimize` | When optimizing test suites, reducing CI wall time, or fixing misplaced coverage. | `.agents/skills/dxrk-test-optimize/SKILL.md` |
| `dxrk-pr-maintainer` | When reviewing, triaging, labeling, or merging PRs/issues. | `.agents/skills/dxrk-pr-maintainer/SKILL.md` |
| `dxrk-release-maintainer` | When preparing, validating, or publishing a release. | `.agents/skills/dxrk-release-maintainer/SKILL.md` |
| `dxrk-qa-testing` | When running QA scenarios, validation gates, or channel tests. | `.agents/skills/dxrk-qa-testing/SKILL.md` |
| `dxrk-secret-scanning` | When triaging or resolving GitHub secret scanning alerts. | `.agents/skills/dxrk-secret-scanning/SKILL.md` |
| `dxrk-ghsa-maintainer` | When inspecting, patching, or publishing GHSA security advisories. | `.agents/skills/dxrk-ghsa-maintainer/SKILL.md` |
| `dxrk-security-triage` | When triaging security reports, drafts, or trust-model verification. | `.agents/skills/dxrk-security-triage/SKILL.md` |
| `dxrk-parallels-smoke` | When running Parallels VM smoke tests (install, onboarding, gateway). | `.agents/skills/dxrk-parallels-smoke/SKILL.md` |
| `dxrk-parallels-e2e` | When running Parallels cross-platform E2E tests with Discord roundtrip. | `.agents/skills/dxrk-parallels-e2e/SKILL.md` |
| `dxrk-pre-release-testing` | When validating pre-release packages, plugins, or artifacts. | `.agents/skills/dxrk-pre-release-testing/SKILL.md` |
| `dxrk-discord-agent` | When communicating via the Discord-backed agent session. | `.agents/skills/dxrk-discord-agent/SKILL.md` |
| `dxrk-testbox` | When running CI-parity checks, secrets management, or hosted service validation. | `.agents/skills/dxrk-testbox/SKILL.md` |
| `dxrk-gitcrawl` | When searching issue/PR archives, finding duplicates, or clustering related threads. | `.agents/skills/dxrk-gitcrawl/SKILL.md` |
| `dxrk-duplicate-detection` | When detecting duplicate PRs/issues and syncing state to GitHub. | `.agents/skills/dxrk-duplicate-detection/SKILL.md` |
| `dxrk-verify` | When validating work, running verification checks, or confirming correctness. | `.agents/skills/dxrk-verify/SKILL.md` |
| `dxrk-remember` | When saving important context, decisions, or findings to Engram memory. | `.agents/skills/dxrk-remember/SKILL.md` |
| `dxrk-debug` | When debugging issues, toggling diagnostics, or troubleshooting failures. | `.agents/skills/dxrk-debug/SKILL.md` |
| `dxrk-simplify` | When explaining complex concepts simply or creating TL;DR summaries. | `.agents/skills/dxrk-simplify/SKILL.md` |
| `dxrk-stuck` | When stuck on a problem and need systematic debugging assistance. | `.agents/skills/dxrk-stuck/SKILL.md` |
| `dxrk-batch` | When batch-processing multiple items or repetitive operations. | `.agents/skills/dxrk-batch/SKILL.md` |
| `dxrk-lorem-ipsum` | When generating placeholder text, test data, or sample content. | `.agents/skills/dxrk-lorem-ipsum/SKILL.md` |
| `dxrk-update-config` | When updating Dxrk configuration, settings, or preferences. | `.agents/skills/dxrk-update-config/SKILL.md` |
| `dxrk-keybindings` | When viewing, modifying, or troubleshooting keybindings. | `.agents/skills/dxrk-keybindings/SKILL.md` |
| `dxrk-skillify` | When saving a conversation or workflow as a reusable skill. | `.agents/skills/dxrk-skillify/SKILL.md` |
| `dxrk-loop` | When running repetitive tasks in a loop with variation. | `.agents/skills/dxrk-loop/SKILL.md` |
| `dxrk-dream` | When doing creative exploration, brainstorming, or open-ended ideation. | `.agents/skills/dxrk-dream/SKILL.md` |
| `dxrk-claude-api` | When interacting with Claude API endpoints or Anthropic services. | `.agents/skills/dxrk-claude-api/SKILL.md` |
| `dxrk-claude-chrome` | When integrating with Chrome extension or browser automation. | `.agents/skills/dxrk-claude-chrome/SKILL.md` |
| `dxrk-api-content` | When managing API content, artifacts, or response formatting. | `.agents/skills/dxrk-api-content/SKILL.md` |
| `dxrk-schedule-agents` | When scheduling remote agent tasks, cron jobs, or automated runs. | `.agents/skills/dxrk-schedule-agents/SKILL.md` |
| `dxrk-skill-generator` | When creating new skills from examples or templates. | `.agents/skills/dxrk-skill-generator/SKILL.md` |
| `react-doctor` | When working on React/JSX/TSX files or reviewing React component health. | `.agents/skills/react-doctor/SKILL.md` |
| `accessibility` | Audit and fix WCAG 2.2, ARIA, and accessibility issues. | `~/.config/opencode/skills/accessibility/SKILL.md` |
| `api-docs` | Generate and maintain API documentation from code, schemas, or OpenAPI specs. | `~/.config/opencode/skills/api-docs/SKILL.md` |
| `arch-decision` | Document architecture decisions as ADRs (context, options, decision, consequences). | `~/.config/opencode/skills/arch-decision/SKILL.md` |
| `ci-cd` | Set up and debug CI/CD pipelines (GitHub Actions, GitLab CI, CircleCI). | `~/.config/opencode/skills/ci-cd/SKILL.md` |
| `code-review` | Review code for security, performance, maintainability, and best practices. | `~/.config/opencode/skills/code-review/SKILL.md` |
| `commit-message` | Generate conventional commits from staged changes. | `~/.config/opencode/skills/commit-message/SKILL.md` |
| `db-query` | Write and optimize SQL, explain plans, and suggest indexes. | `~/.config/opencode/skills/db-query/SKILL.md` |
| `debugging` | Systematic debugging workflow — hypothesis, isolate, fix, verify. | `~/.config/opencode/skills/debugging/SKILL.md` |
| `dependency` | Manage dependencies — audit, update, migrate, vendor, secure. | `~/.config/opencode/skills/dependency/SKILL.md` |
| `docker-mgmt` | Create and optimize Dockerfiles, compose files, and container workflows. | `~/.config/opencode/skills/docker-mgmt/SKILL.md` |
| `env-setup` | Configure dev environments — .env files, secrets management, local services. | `~/.config/opencode/skills/env-setup/SKILL.md` |
| `error-handling` | Add proper error handling — typed errors, recovery, user-facing messages. | `~/.config/opencode/skills/error-handling/SKILL.md` |
| `git-release` | Create consistent releases with changelogs, version bumps, and git tags. | `~/.config/opencode/skills/git-release/SKILL.md` |
| `logging-patterns` | Add structured logging with appropriate levels, context, and best practices. | `~/.config/opencode/skills/logging-patterns/SKILL.md` |
| `migration` | Plan and execute safe database and code migrations. | `~/.config/opencode/skills/migration/SKILL.md` |
| `performance` | Profile and optimize code — identify bottlenecks, reduce latency. | `~/.config/opencode/skills/performance/SKILL.md` |
| `pr-description` | Generate structured PR descriptions from diffs. | `~/.config/opencode/skills/pr-description/SKILL.md` |
| `refactoring-pr` | Plan and execute refactoring while preserving behavior. | `~/.config/opencode/skills/refactoring-pr/SKILL.md` |
| `security-audit` | Scan code for OWASP Top 10, injection flaws, hardcoded secrets, and vulns. | `~/.config/opencode/skills/security-audit/SKILL.md` |
| `test-writer` | Write tests following project patterns — unit, integration, e2e. | `~/.config/opencode/skills/test-writer/SKILL.md` |
