// SPDX-License-Identifier: MIT
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/agents/kimi"
	"github.com/Dxrk777/Dxrk/internal/assets"
	"github.com/Dxrk777/Dxrk/internal/backup"
	"github.com/Dxrk777/Dxrk/internal/components/checker"
	"github.com/Dxrk777/Dxrk/internal/components/dxrkguardian"
	"github.com/Dxrk777/Dxrk/internal/components/dxrkmemory"
	"github.com/Dxrk777/Dxrk/internal/components/internalmcp"
	"github.com/Dxrk777/Dxrk/internal/components/mcp"
	"github.com/Dxrk777/Dxrk/internal/components/opencodeplugin"
	"github.com/Dxrk777/Dxrk/internal/components/permissions"
	"github.com/Dxrk777/Dxrk/internal/components/persona"
	"github.com/Dxrk777/Dxrk/internal/components/sdd"
	"github.com/Dxrk777/Dxrk/internal/components/skills"
	"github.com/Dxrk777/Dxrk/internal/components/theme"
	"github.com/Dxrk777/Dxrk/internal/installcmd"
	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/pipeline"
	"github.com/Dxrk777/Dxrk/internal/planner"
	"github.com/Dxrk777/Dxrk/internal/state"
	"github.com/Dxrk777/Dxrk/internal/system"
	"github.com/Dxrk777/Dxrk/internal/verify"
)

type InstallResult struct {
	Selection    model.Selection
	Resolved     planner.ResolvedPlan
	Review       planner.ReviewPayload
	Plan         pipeline.StagePlan
	Execution    pipeline.ExecutionResult
	Verify       verify.Report
	Dependencies system.DependencyReport
	DryRun       bool
}

var (
	osUserHomeDir       = os.UserHomeDir
	osSetenv            = os.Setenv
	osStat              = os.Stat
	runCommand          = executeCommand
	cmdLookPath         = exec.LookPath
	streamCommandOutput = true

	// dxrkGuardianAvailableCheck is an optional override for dxrkGuardianAvailable behavior.
	// When set, it is called instead of the default filesystem check.
	dxrkGuardianAvailableCheck func(system.PlatformProfile) bool

	// dxrkMemoryDownloadFn is the function used to download the dxrk-memory binary on non-brew platforms.
	// Package-level var for testability — tests can replace this to avoid real HTTP calls.
	dxrkMemoryDownloadFn = dxrkmemory.DownloadLatestBinary

	// AppVersion is the dxrk version that will be written into backup manifests.
	// It is set by app.go before any CLI operation so that every backup created during
	// an install or sync records which version of dxrk made it.
	// Default "dev" matches the ldflags default in app.Version.
	AppVersion = "dev"
)

// SetCommandOutputStreaming toggles whether command stdout/stderr is streamed
// directly to the terminal. It returns a restore function.
func SetCommandOutputStreaming(enabled bool) func() {
	previous := streamCommandOutput
	streamCommandOutput = enabled
	return func() {
		streamCommandOutput = previous
	}
}

func RunInstall(args []string, detection system.DetectionResult) (InstallResult, error) {
	flags, err := ParseInstallFlags(args)
	if err != nil {
		return InstallResult{}, err
	}

	input, err := NormalizeInstallFlags(flags, detection)
	if err != nil {
		return InstallResult{}, err
	}

	resolved, err := planner.NewResolver(planner.MVPGraph()).Resolve(input.Selection)
	if err != nil {
		return InstallResult{}, err
	}
	profile := ResolveInstallProfile(detection)
	resolved.PlatformDecision = planner.PlatformDecisionFromProfile(profile)

	review := planner.BuildReviewPayload(input.Selection, resolved)
	stagePlan := buildStagePlan(input.Selection, resolved)

	result := InstallResult{
		Selection:    input.Selection,
		Resolved:     resolved,
		Review:       review,
		Plan:         stagePlan,
		Dependencies: detection.Dependencies,
		DryRun:       input.DryRun,
	}

	if input.DryRun {
		return result, nil
	}

	homeDir, err := osUserHomeDir()
	if err != nil {
		return result, fmt.Errorf("resolve user home directory: %w", err)
	}

	installRuntime, err := newInstallRuntime(homeDir, input.Selection, resolved, profile)
	if err != nil {
		return result, err
	}

	// Print dependency warnings before the pipeline starts (CLI only).
	// The TUI surfaces these on the complete screen instead.
	if !detection.Dependencies.AllPresent {
		fmt.Fprintf(os.Stderr, "WARNING: missing dependencies: %s\n\n%s\n",
			strings.Join(detection.Dependencies.MissingRequired, ", "),
			system.FormatMissingDepsMessage(detection.Dependencies))
	}

	stagePlan = installRuntime.stagePlan()
	result.Plan = stagePlan

	orchestrator := pipeline.NewOrchestrator(pipeline.DefaultRollbackPolicy())
	result.Execution = orchestrator.Execute(stagePlan)
	if result.Execution.Err != nil {
		return result, fmt.Errorf("execute install pipeline: %w", result.Execution.Err)
	}

	result.Verify = runPostApplyVerification(homeDir, installRuntime.workspaceDir, input.Selection, resolved)
	result.Verify = withPostInstallNotes(result.Verify, resolved)
	if !result.Verify.Ready {
		return result, fmt.Errorf("post-apply verification failed:\n%s", verify.RenderReport(result.Verify))
	}

	// Persist the user's agent selection and model assignments so that future
	// `sync` runs target only the installed agents and preserve model choices.
	agentIDs := make([]string, 0, len(input.Selection.Agents))
	for _, a := range input.Selection.Agents {
		agentIDs = append(agentIDs, string(a))
	}
	// Non-fatal: a state write failure must not break an otherwise successful install.
	_ = state.Write(homeDir, state.InstallState{
		InstalledAgents:        agentIDs,
		ClaudeModelAssignments: claudeAliasesToStrings(input.Selection.ClaudeModelAssignments),
		ModelAssignments:       modelAssignmentsToState(input.Selection.ModelAssignments),
		Persona:                string(input.Selection.Persona),
	})

	return result, nil
}

func withPostInstallNotes(report verify.Report, resolved planner.ResolvedPlan) verify.Report {
	if hasComponent(resolved.OrderedComponents, model.ComponentDxrkGuardian) && report.Ready {
		report.FinalNote += "\n\nGGA is now installed globally. To enable project hooks, run in each repo:\n- dxrk-guardian init\n- dxrk-guardian install"
	}
	report = withGoInstallPathNote(report, resolved)
	return report
}

// withGoInstallPathNote appends a PATH guidance note when dxrk-memory was installed
// on a non-brew platform (Linux/Windows). Since dxrk-memory is now installed via
// direct binary download to /usr/local/bin or ~/.local/bin, this note helps
// users who may need to add the install directory to their PATH.
func withGoInstallPathNote(report verify.Report, resolved planner.ResolvedPlan) verify.Report {
	if !hasComponent(resolved.OrderedComponents, model.ComponentDxrkMemory) {
		return report
	}
	if resolved.PlatformDecision.PackageManager == "brew" {
		return report
	}
	binDir := goInstallBinDir()
	if isInPATH(binDir) {
		return report
	}
	report.FinalNote += fmt.Sprintf(
		"\n\nThe dxrk-memory binary was installed to %s via `go install`.\nAdd it to your PATH: %s",
		binDir,
		dxrkMemoryPathGuidance(os.Getenv("SHELL")),
	)
	return report
}

// goInstallBinDir returns the directory where `go install` places binaries.
// Resolution order: $GOBIN > $GOPATH/bin > $HOME/go/bin.
func goInstallBinDir() string {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return gobin
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	if home, err := osUserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin")
	}
	return filepath.Join("~", "go", "bin")
}

// isInPATH reports whether dir is present in the current PATH.
func isInPATH(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == dir {
			return true
		}
	}
	return false
}

func buildStagePlan(selection model.Selection, resolved planner.ResolvedPlan) pipeline.StagePlan {
	prepare := []pipeline.Step{
		noopStep{id: "prepare:system-check"},
		noopStep{id: "prepare:check-dependencies"},
	}
	apply := make([]pipeline.Step, 0, len(resolved.Agents)+len(resolved.OrderedComponents))

	for _, agent := range resolved.Agents {
		apply = append(apply, noopStep{id: "agent:" + string(agent)})
	}

	for _, component := range resolved.OrderedComponents {
		apply = append(apply, noopStep{id: "component:" + string(component)})
	}

	if len(selection.Agents) == 0 && len(resolved.OrderedComponents) == 0 {
		prepare = nil
	}

	return pipeline.StagePlan{Prepare: prepare, Apply: apply}
}

type installRuntime struct {
	homeDir      string
	workspaceDir string
	selection    model.Selection
	resolved     planner.ResolvedPlan
	profile      system.PlatformProfile
	backupRoot   string
	state        *runtimeState
}

type runtimeState struct {
	manifest backup.Manifest
}

func newInstallRuntime(homeDir string, selection model.Selection, resolved planner.ResolvedPlan, profile system.PlatformProfile) (*installRuntime, error) {
	backupRoot := filepath.Join(homeDir, ".dxrk", "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create backup root directory %q: %w", backupRoot, err)
	}

	workspaceDir, _ := os.Getwd()
	workspaceDir = resolveOpenClawWorkspaceDir(homeDir, workspaceDir, resolved.Agents)

	return &installRuntime{
		homeDir:      homeDir,
		workspaceDir: workspaceDir,
		selection:    selection,
		resolved:     resolved,
		profile:      profile,
		backupRoot:   backupRoot,
		state:        &runtimeState{},
	}, nil
}

func (r *installRuntime) stagePlan() pipeline.StagePlan {
	targets := backupTargets(r.homeDir, r.workspaceDir, r.selection, r.resolved)
	prepare := []pipeline.Step{
		checkDependenciesStep{id: "prepare:check-dependencies", profile: r.profile, homeDir: r.homeDir, selection: r.selection},
		prepareBackupStep{
			id:          "prepare:backup-snapshot",
			snapshotter: backup.NewSnapshotter(),
			snapshotDir: filepath.Join(r.backupRoot, time.Now().UTC().Format("20060102150405.000000000")),
			targets:     targets,
			state:       r.state,
			backupRoot:  r.backupRoot,
			source:      backup.BackupSourceInstall,
			description: "pre-install snapshot",
			appVersion:  AppVersion,
		},
	}

	apply := make([]pipeline.Step, 0, len(r.resolved.Agents)+len(r.resolved.OrderedComponents)+1)
	apply = append(apply, rollbackRestoreStep{id: "apply:rollback-restore", state: r.state})

	// Before installing components, ensure modular agents have their system prompt hub.
	// This ensures that SDD or DxrkMemory can inject their modules even if Persona is skipped.
	for _, agent := range r.resolved.Agents {
		if agent == model.AgentKimi {
			apply = append(apply, kimiSystemPromptHubStep{id: "agent:kimi-prompt-hub", homeDir: r.homeDir})
		}
	}

	for _, agent := range r.resolved.Agents {
		apply = append(apply, agentInstallStep{id: "agent:" + string(agent), agent: agent, homeDir: r.homeDir, profile: r.profile})
	}

	if containsAgent(r.resolved.Agents, model.AgentOpenCode) {
		for _, plugin := range r.selection.OpenCodePlugins {
			apply = append(apply, openCodePluginInstallStep{id: "opencode-plugin:" + string(plugin), plugin: plugin, homeDir: r.homeDir})
		}
	}

	for _, component := range r.resolved.OrderedComponents {
		apply = append(apply, componentApplyStep{
			id:           "component:" + string(component),
			component:    component,
			homeDir:      r.homeDir,
			workspaceDir: r.workspaceDir,
			agents:       r.resolved.Agents,
			selection:    r.selection,
			profile:      r.profile,
		})
	}

	return pipeline.StagePlan{Prepare: prepare, Apply: apply}
}

type prepareBackupStep struct {
	id          string
	snapshotter backup.Snapshotter
	snapshotDir string
	targets     []string
	state       *runtimeState

	// backupRoot is the parent directory of all backup snapshots.
	// When set, deduplication (IsDuplicate) and retention pruning (Prune) are
	// enabled. When empty, both are skipped (backward-compatible default).
	backupRoot string

	// source and description are optional metadata written into the manifest.
	// When set, they help users identify what created the backup.
	source      backup.BackupSource
	description string

	// appVersion is the dxrk version that created this backup.
	// When set, it is written into the manifest as CreatedByVersion.
	appVersion string
}

func (s prepareBackupStep) ID() string {
	return s.id
}

func (s prepareBackupStep) Run() error {
	// Deduplication: skip snapshot creation when content is identical to the
	// most recent backup. Only active when backupRoot is set.
	if s.backupRoot != "" {
		checksum, err := backup.ComputeChecksum(s.targets)
		if err == nil && checksum != "" {
			if dup, dupErr := backup.IsDuplicate(s.backupRoot, checksum); dupErr != nil {
				log.Printf("backup: check duplicate: %v", dupErr)
			} else if dup {
				// Content is identical to the most recent backup — skip creation.
				// state.manifest is left at its zero value; rollback is a no-op.
				return nil
			}
		}
	}

	manifest, err := s.snapshotter.Create(s.snapshotDir, s.targets)
	if err != nil {
		return fmt.Errorf("create backup snapshot: %w", err)
	}

	// Annotate with source metadata and version when provided, then re-write.
	// FileCount is already populated by Snapshotter.Create.
	if s.source != "" || s.appVersion != "" {
		manifest.Source = s.source
		manifest.Description = s.description
		manifest.CreatedByVersion = s.appVersion
		manifestPath := filepath.Join(s.snapshotDir, backup.ManifestFilename)
		if err := backup.WriteManifest(manifestPath, manifest); err != nil {
			// Non-fatal: metadata annotation failed but the snapshot is intact.
			// The backup is still usable — restore will work. We just lose the label.
			log.Printf("backup: annotate manifest: %v", err)
		}
	}

	s.state.manifest = manifest

	// Retention pruning: remove oldest unpinned backups beyond the limit.
	// Non-fatal: a prune failure must not prevent the install/sync from succeeding.
	if s.backupRoot != "" {
		if _, pruneErr := backup.Prune(s.backupRoot, backup.DefaultRetentionCount); pruneErr != nil {
			log.Printf("backup: prune: %v", pruneErr)
		}
	}

	return nil
}

type rollbackRestoreStep struct {
	id    string
	state *runtimeState
}

func (s rollbackRestoreStep) ID() string {
	return s.id
}

func (s rollbackRestoreStep) Run() error {
	return nil
}

func (s rollbackRestoreStep) Rollback() error {
	if len(s.state.manifest.Entries) == 0 {
		return nil
	}

	return backup.RestoreService{}.Restore(s.state.manifest)
}

type agentInstallStep struct {
	id      string
	agent   model.AgentID
	homeDir string
	profile system.PlatformProfile
}

type openCodePluginInstallStep struct {
	id      string
	plugin  model.OpenCodeCommunityPluginID
	homeDir string
}

func (s openCodePluginInstallStep) ID() string { return s.id }

func (s openCodePluginInstallStep) Run() error {
	_, err := opencodeplugin.Install(s.homeDir, s.plugin)
	return err
}

func (s agentInstallStep) ID() string {
	return s.id
}

func (s agentInstallStep) Run() error {
	adapter, err := agents.NewAdapter(s.agent)
	if err != nil {
		return fmt.Errorf("create adapter for %q: %w", s.agent, err)
	}

	if !adapter.SupportsAutoInstall() {
		return nil
	}

	installed, _, _, _, err := adapter.Detect(context.Background(), s.homeDir)
	if err != nil {
		return fmt.Errorf("detect agent %q: %w", s.agent, err)
	}
	if installed && s.agent != model.AgentPi {
		return nil
	}

	if err := installcmd.ValidateAgentInstallPreflight(s.profile, s.agent); err != nil {
		return fmt.Errorf("preflight for agent %q: %w", s.agent, err)
	}

	commands, err := adapter.InstallCommand(s.profile)
	if err != nil {
		return fmt.Errorf("resolve install command for %q: %w", s.agent, err)
	}
	if len(commands) == 0 {
		return fmt.Errorf("install command for %q resolved to an empty sequence (unsupported platform or resolver misconfiguration)", s.agent)
	}

	return runCommandSequence(commands)
}

type kimiSystemPromptHubStep struct {
	id      string
	homeDir string
}

func (s kimiSystemPromptHubStep) ID() string {
	return s.id
}

func (s kimiSystemPromptHubStep) Run() error {
	return kimi.NewAdapter().BootstrapTemplate(s.homeDir)
}

type componentApplyStep struct {
	id           string
	component    model.ComponentID
	homeDir      string
	workspaceDir string
	agents       []model.AgentID
	selection    model.Selection
	profile      system.PlatformProfile
}

func (s componentApplyStep) ID() string {
	return s.id
}

// resolveAdapters creates adapters for each agent ID, skipping unsupported ones.
func resolveAdapters(agentIDs []model.AgentID) []agents.Adapter {
	adapters := make([]agents.Adapter, 0, len(agentIDs))
	for _, id := range agentIDs {
		adapter, err := agents.NewAdapter(id)
		if err != nil {
			continue
		}
		adapters = append(adapters, adapter)
	}
	return adapters
}

func (s componentApplyStep) Run() error {
	adapters := resolveAdapters(s.agents)

	switch s.component {
	case model.ComponentDxrkMemory:
		if _, err := cmdLookPath("dxrk-memory"); err != nil {
			// DxrkMemory not on PATH — install it.
			if s.profile.PackageManager == "brew" {
				// macOS (or Linux with Homebrew): use brew tap + brew install.
				commands, err := dxrkmemory.InstallCommand(s.profile)
				if err != nil {
					return fmt.Errorf("resolve install command for component %q: %w", s.component, err)
				}
				if err := runCommandSequence(commands); err != nil {
					return err
				}
			} else {
				// Linux / Windows: download the pre-built binary from GitHub Releases.
				// No Go required — dxrk-memory ships pre-built binaries.
				binaryPath, err := dxrkMemoryDownloadFn(s.profile)
				if err != nil {
					return fmt.Errorf("download dxrk-memory binary: %w", err)
				}
				// Add the install directory to PATH so subsequent commands
				// (dxrk-memory setup, dxrkmemory.Inject → resolveDxrkMemoryCommand) can find it.
				// On Windows this also persists the change to the user registry via PowerShell.
				binDir := filepath.Dir(binaryPath)
				if err := system.AddToUserPath(binDir); err != nil {
					// Non-fatal: warn but continue — the binary was downloaded successfully.
					fmt.Fprintf(os.Stderr, "WARNING: could not add %s to PATH: %v\n", binDir, err)
				}
			}
		}
		setupMode := dxrkmemory.ParseSetupMode(os.Getenv(dxrkmemory.SetupModeEnvVar))
		setupStrict := dxrkmemory.ParseSetupStrict(os.Getenv(dxrkmemory.SetupStrictEnvVar))
		attemptedSlugs := make(map[string]struct{}, len(adapters))
		for _, adapter := range adapters {
			if dxrkmemory.ShouldAttemptSetup(setupMode, adapter.Agent()) {
				slug, _ := dxrkmemory.SetupAgentSlug(adapter.Agent())
				if _, seen := attemptedSlugs[slug]; !seen {
					if err := runCommand("dxrk-memory", "setup", slug); err != nil {
						if setupStrict {
							return fmt.Errorf("dxrk-memory setup for %q: %w", adapter.Agent(), err)
						}
					}
					attemptedSlugs[slug] = struct{}{}
				}
			}
			var err error
			if adapter.Agent() == model.AgentOpenClaw {
				_, err = dxrkmemory.InjectWithPromptDir(s.homeDir, s.workspaceDir, adapter)
			} else {
				targetDir := componentInjectionDir(s.homeDir, s.workspaceDir, adapter)
				_, err = dxrkmemory.Inject(targetDir, adapter)
			}
			if err != nil {
				return fmt.Errorf("inject dxrk-memory for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentContext7:
		for _, adapter := range adapters {
			if _, err := mcp.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject context7 for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentPersona:
		for _, adapter := range adapters {
			targetDir := componentInjectionDir(s.homeDir, s.workspaceDir, adapter)
			if _, err := persona.Inject(targetDir, adapter, s.selection.Persona); err != nil {
				return fmt.Errorf("inject persona for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentPermission:
		for _, adapter := range adapters {
			if _, err := permissions.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject permissions for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentSDD:
		for _, adapter := range adapters {
			targetDir := componentInjectionDir(s.homeDir, s.workspaceDir, adapter)
			opts := sdd.InjectOptions{
				OpenCodeModelAssignments: s.selection.ModelAssignments,
				ClaudeModelAssignments:   s.selection.ClaudeModelAssignments,
				KiroModelAssignments:     s.selection.KiroModelAssignments,
				WorkspaceDir:             s.workspaceDir,
				StrictTDD:                s.selection.StrictTDD,
			}
			if _, err := sdd.Inject(targetDir, adapter, s.selection.SDDMode, opts); err != nil {
				return fmt.Errorf("inject sdd for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentSkills:
		skillIDs := selectedSkillIDs(s.selection)
		if len(skillIDs) == 0 {
			return nil
		}
		for _, adapter := range adapters {
			if _, err := skills.Inject(s.homeDir, adapter, skillIDs); err != nil {
				return fmt.Errorf("inject skills for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentDxrkGuardian:
		if !dxrkGuardianAvailable(s.profile) {
			// Dxrk Guardian not found on any known PATH — install it.
			commands, err := dxrkguardian.InstallCommand(s.profile)
			if err != nil {
				return fmt.Errorf("resolve install command for component %q: %w", s.component, err)
			}
			installErr := runCommandSequence(commands)
			if installErr != nil {
				if dxrkGuardianAvailable(s.profile) {
					// The Dxrk Guardian install script uses `set -e` and `read -p` for
					// the "already installed" confirmation. Without a TTY
					// (common in automated/re-run scenarios), `read` fails
					// with exit code 1 and `set -e` kills the script before
					// it can exit 0. If Dxrk Guardian is actually available after the
					// script ran, the install succeeded functionally — treat
					// as success but warn the user.
					fmt.Fprintf(os.Stderr, "WARNING: dxrk-guardian install command reported an error but dxrk-guardian is available — continuing. Error was: %v\n", installErr)
				} else {
					return installErr
				}
			}
		}
		if err := dxrkguardian.EnsureRuntimeAssets(s.homeDir); err != nil {
			return fmt.Errorf("ensure dxrk-guardian runtime assets: %w", err)
		}
		if goruntime.GOOS == goosWindows {
			if err := dxrkguardian.EnsurePowerShellShim(s.homeDir); err != nil {
				return fmt.Errorf("ensure dxrk-guardian powershell shim: %w", err)
			}
			// Add Dxrk Guardian bin dir to the user PATH persistently on Windows.
			// Dxrk Guardian's install.sh drops the binary into ~/bin which is not on PATH by default.
			dxrkGuardianBinDir := filepath.Join(s.homeDir, "bin")
			if err := system.AddToUserPath(dxrkGuardianBinDir); err != nil {
				// Non-fatal: warn but continue — Dxrk Guardian was installed successfully.
				fmt.Fprintf(os.Stderr, "WARNING: could not add %s to PATH: %v\n", dxrkGuardianBinDir, err)
			}
		}
		if _, err := dxrkguardian.Inject(s.homeDir, s.agents); err != nil {
			return fmt.Errorf("inject dxrk-guardian config: %w", err)
		}
		return nil
	case model.ComponentTheme:
		for _, adapter := range adapters {
			if _, err := theme.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject theme for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentClaudeTheme:
		for _, adapter := range adapters {
			if _, err := theme.InjectClaudeTheme(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject Claude theme for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentOpenCodeDxrkLogo:
		if _, err := opencodeplugin.Install(s.homeDir, model.OpenCodePluginDxrkLogo); err != nil {
			return fmt.Errorf("install OpenCode Dxrk Logo plugin: %w", err)
		}
		return nil
	case model.ComponentChecker:
		if _, err := checker.Inject(s.homeDir); err != nil {
			return fmt.Errorf("inject checker: %w", err)
		}
		return nil
	case model.ComponentInternalMCPServer:
		for _, adapter := range adapters {
			if _, err := internalmcp.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject internal MCP server for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	default:
		return fmt.Errorf("component %q is not supported in install runtime", s.component)
	}
}

func ensureGoAvailableAfterInstall(profile system.PlatformProfile) error {
	if _, err := cmdLookPath("go"); err == nil {
		return nil
	}

	if profile.OS != goosWindows {
		return fmt.Errorf("go was installed but is still not available in PATH")
	}

	for _, candidate := range windowsGoCandidates() {
		if candidate == "" {
			continue
		}
		if _, err := osStat(candidate); err == nil {
			binDir := filepath.Dir(candidate)
			currentPath := os.Getenv("PATH")
			if currentPath == "" {
				return osSetenv("PATH", binDir)
			}
			return osSetenv("PATH", binDir+string(os.PathListSeparator)+currentPath)
		}
	}

	return fmt.Errorf("go was installed but is still not available in PATH; restart the terminal and retry")
}

func windowsGoCandidates() []string {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	return []string{
		filepath.Join(programFiles, "Go", "bin", "go.exe"),
		filepath.Join(programFilesX86, "Go", "bin", "go.exe"),
		`C:\Program Files\Go\bin\go.exe`,
	}
}

// BuildRealStagePlan creates a StagePlan with real backup, agent install, and component apply steps.
// It is used by both the CLI and TUI paths.
func BuildRealStagePlan(homeDir string, selection model.Selection, resolved planner.ResolvedPlan, profile system.PlatformProfile) (pipeline.StagePlan, error) {
	backupRoot := filepath.Join(homeDir, ".dxrk", "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		return pipeline.StagePlan{}, fmt.Errorf("create backup root directory %q: %w", backupRoot, err)
	}

	installRuntime, err := newInstallRuntime(homeDir, selection, resolved, profile)
	if err != nil {
		return pipeline.StagePlan{}, err
	}

	return installRuntime.stagePlan(), nil
}

// ResolveInstallProfile returns the platform profile from detection, defaulting to darwin/brew.
func ResolveInstallProfile(detection system.DetectionResult) system.PlatformProfile {
	if detection.System.Profile.OS != "" {
		return detection.System.Profile
	}

	return system.PlatformProfile{
		OS:             "darwin",
		PackageManager: "brew",
		Supported:      true,
	}
}

// dxrkGuardianAvailable reports whether the dxrk-guardian binary is reachable. dxrk-guardian is often
// installed to ~/.local/bin (the default for install.sh on Linux and macOS)
// or ~/bin (the default for install.sh on Windows), which may not be on PATH.
// On macOS with Homebrew, dxrk-guardian may be in /opt/homebrew/bin or /usr/local/bin.
// We check the filesystem directly to avoid spawning a subprocess and to work
// regardless of whether the install directory has been added to PATH.
func dxrkGuardianAvailable(profile system.PlatformProfile) bool {
	// Allow test override.
	if dxrkGuardianAvailableCheck != nil {
		return dxrkGuardianAvailableCheck(profile)
	}
	if _, err := cmdLookPath("dxrk-guardian"); err == nil {
		return true
	}
	homeDir, err := osUserHomeDir()
	if err != nil {
		return false
	}
	if _, err := osStat(filepath.Join(homeDir, ".local", "bin", "dxrk-guardian")); err == nil {
		return true
	}
	// Check well-known Homebrew prefixes for macOS (arm64 and x86).
	// dxrk-guardian may be installed via brew but not yet in the shell PATH
	// (e.g. new terminal session, Rosetta environment mismatch).
	if profile.OS == "darwin" || profile.PackageManager == "brew" {
		for _, brewBin := range []string{
			"/opt/homebrew/bin/dxrk-guardian",
			"/usr/local/bin/dxrk-guardian",
		} {
			if _, err := osStat(brewBin); err == nil {
				return true
			}
		}
	}
	if profile.OS == goosWindows {
		if _, err := osStat(filepath.Join(homeDir, "bin", "dxrk-guardian")); err == nil {
			return true
		}
	}
	return false
}

// runCommandSequence runs each command in the sequence one at a time, stopping on first error.
func runCommandSequence(commands [][]string) error {
	if len(commands) == 0 {
		return fmt.Errorf("empty command sequence")
	}

	for _, command := range commands {
		if len(command) == 0 {
			return fmt.Errorf("empty command in sequence")
		}

		if err := runCommand(command[0], command[1:]...); err != nil {
			return fmt.Errorf("run command %q: %w", strings.Join(command, " "), err)
		}
	}

	return nil
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec

	if streamCommandOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%w\noutput:\n%s", err, strings.TrimSpace(string(output)))
		}
		return err
	}

	return nil
}

// selectedSkillIDs returns the skill IDs to install. If the selection
// has explicit skills, those are used; otherwise skills are derived from the preset.
func selectedSkillIDs(selection model.Selection) []model.SkillID {
	if len(selection.Skills) > 0 {
		return selection.Skills
	}

	return skills.SkillsForPreset(selection.Preset)
}

func backupTargets(homeDir, workspaceDir string, selection model.Selection, resolved planner.ResolvedPlan) []string {
	paths := map[string]struct{}{}
	adapters := resolveAdapters(resolved.Agents)

	for _, component := range resolved.OrderedComponents {
		for _, path := range componentPathsWithWorkspace(homeDir, workspaceDir, selection, adapters, component) {
			paths[path] = struct{}{}
		}
	}

	targets := make([]string, 0, len(paths))
	for path := range paths {
		targets = append(targets, path)
	}

	return targets
}

func componentPaths(homeDir string, selection model.Selection, adapters []agents.Adapter, component model.ComponentID) []string {
	return componentPathsWithWorkspace(homeDir, "", selection, adapters, component)
}

func componentPathsWithWorkspace(homeDir, workspaceDir string, selection model.Selection, adapters []agents.Adapter, component model.ComponentID) []string {
	paths := []string{}
	for _, adapter := range adapters {
		targetDir := componentPathDir(homeDir, workspaceDir, adapter, component)
		switch component {
		case model.ComponentDxrkMemory:
			switch adapter.MCPStrategy() {
			case model.StrategySeparateMCPFiles:
				paths = append(paths, adapter.MCPConfigPath(targetDir, "dxrk-memory"))
			case model.StrategyMergeIntoSettings:
				// MCP settings are always merged into the global config file, not the
				// workspace-scoped directory. For OpenClaw, SettingsPath(targetDir)
				// would yield <workspace>/.openclaw/openclaw.json, but dxrk-memory injection
				// writes to the canonical ~/.openclaw/openclaw.json (homeDir). Use
				// homeDir here so the verification path matches the actual write target.
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyMCPConfigFile:
				if p := adapter.MCPConfigPath(targetDir, "dxrk-memory"); p != "" {
					paths = append(paths, p)
				}
				if adapter.Agent() == model.AgentAntigravity {
					if p := adapter.SettingsPath(homeDir); p != "" {
						paths = append(paths, p)
					}
				}
			case model.StrategyTOMLFile:
				if p := adapter.MCPConfigPath(targetDir, "dxrk-memory"); p != "" {
					paths = append(paths, p)
				}
			}
			if adapter.SystemPromptStrategy() == model.StrategyMarkdownSections {
				paths = append(paths, adapter.SystemPromptFile(targetDir))
			}
		case model.ComponentSDD:
			// Jinja modular hubs (e.g. Kimi KIMI.md) are appended once below so SDD+Persona
			// do not duplicate the same system prompt path.
			if adapter.SupportsSystemPrompt() && adapter.SystemPromptStrategy() != model.StrategyJinjaModules {
				paths = append(paths, adapter.SystemPromptFile(targetDir))
			}
			if adapter.SupportsSlashCommands() {
				for _, command := range sdd.OpenCodeCommands() {
					paths = append(paths, filepath.Join(adapter.CommandsDir(homeDir), command.Name+".md"))
				}
			}
			if adapter.Agent() == model.AgentOpenCode {
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
				paths = append(paths,
					filepath.Join(homeDir, ".config", "opencode", "plugins", "background-agents.ts"),
					filepath.Join(homeDir, ".config", "opencode", "plugins", "model-variants.ts"),
				)
				// Shared prompt files in ~/.config/opencode/prompts/sdd/ — back these up
				// so a sync does not silently overwrite user-customized prompt content.
				// These files are only written for multi-mode (SDDModeMulti), so we only
				// include them in the path list when that mode is active. This prevents
				// false-negative verification failures in single/empty mode syncs.
				if selection.SDDMode == model.SDDModeMulti {
					promptDir := sdd.SharedPromptDir(homeDir)
					for _, phase := range sdd.SharedPromptPhases() {
						paths = append(paths, filepath.Join(promptDir, phase+".md"))
					}
				}
			}
			if adapter.SupportsSkills() {
				skillDir := adapter.SkillsDir(targetDir)
				if skillDir != "" {
					paths = append(paths,
						filepath.Join(skillDir, "_shared", "persistence-contract.md"),
						filepath.Join(skillDir, "_shared", "dxrk-memory-convention.md"),
						filepath.Join(skillDir, "_shared", "openspec-convention.md"),
						filepath.Join(skillDir, "_shared", "sdd-phase-common.md"),
						filepath.Join(skillDir, "_shared", "skill-resolver.md"),
						filepath.Join(skillDir, "sdd-init", "SKILL.md"),
						filepath.Join(skillDir, "sdd-explore", "SKILL.md"),
						filepath.Join(skillDir, "sdd-propose", "SKILL.md"),
						filepath.Join(skillDir, "sdd-spec", "SKILL.md"),
						filepath.Join(skillDir, "sdd-design", "SKILL.md"),
						filepath.Join(skillDir, "sdd-tasks", "SKILL.md"),
						filepath.Join(skillDir, "sdd-apply", "SKILL.md"),
						filepath.Join(skillDir, "sdd-verify", "SKILL.md"),
						filepath.Join(skillDir, "sdd-archive", "SKILL.md"),
					)
				}
			}
			paths = append(paths, sddSubAgentPaths(homeDir, adapter)...)
		case model.ComponentSkills:
			for _, skillID := range selectedSkillIDs(selection) {
				if skills.IsSDDSkill(skillID) {
					continue
				}
				path := skills.SkillPathForAgent(homeDir, adapter, skillID)
				if path != "" {
					paths = append(paths, path)
				}
			}
		case model.ComponentContext7:
			switch adapter.MCPStrategy() {
			case model.StrategySeparateMCPFiles:
				paths = append(paths, adapter.MCPConfigPath(homeDir, "context7"))
			case model.StrategyMergeIntoSettings:
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyMCPConfigFile:
				if p := adapter.MCPConfigPath(homeDir, "context7"); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyTOMLFile:
				// Codex uses TOML for DxrkMemory but Context7 is not injected via TOML.
				// No path to report — Context7 injection is skipped for TOML agents.
			}
		case model.ComponentPersona:
			if selection.Persona == model.PersonaCustom {
				break
			}
			if adapter.Agent() == model.AgentOpenClaw {
				paths = append(paths, filepath.Join(targetDir, "SOUL.md"))
				break
			}
			if adapter.SupportsSystemPrompt() && adapter.SystemPromptStrategy() != model.StrategyJinjaModules {
				paths = append(paths, adapter.SystemPromptFile(targetDir))
			}
			if selection.Persona == model.PersonaDxrk {
				if adapter.SupportsOutputStyles() {
					paths = append(paths, adapter.OutputStyleDir(targetDir)+"/dxrk.md")
					if p := adapter.SettingsPath(targetDir); p != "" {
						paths = append(paths, p)
					}
				}
			}
		case model.ComponentPermission:
			if p := adapter.SettingsPath(homeDir); p != "" {
				paths = append(paths, p)
			}
		case model.ComponentDxrkGuardian:
			paths = append(paths, dxrkguardian.ConfigPath(homeDir))
			paths = append(paths, dxrkguardian.AgentsTemplatePath(homeDir))
		case model.ComponentTheme:
			if p := adapter.SettingsPath(homeDir); p != "" {
				paths = append(paths, p)
			}
		case model.ComponentClaudeTheme:
			if adapter.Agent() == model.AgentClaudeCode {
				paths = append(paths, filepath.Join(homeDir, ".claude", "themes", "dxrk.json"))
			}
		case model.ComponentOpenCodeDxrkLogo:
			paths = append(paths,
				filepath.Join(homeDir, ".config", "opencode", "tui-plugins", "dxrk-logo.tsx"),
				filepath.Join(homeDir, ".config", "opencode", "tui.json"),
			)
		}
	}

	// Always ensure the main system prompt file is included for verification if the agent
	// supports modular system prompts (like Kimi), even if no specific component
	// (like Persona) was selected. This prevents false negatives when the skeleton
	// is bootstrapped but not explicitly owned by any other component path list.
	for _, adapter := range adapters {
		if adapter.SystemPromptStrategy() == model.StrategyJinjaModules {
			paths = append(paths, adapter.SystemPromptFile(homeDir))
		}
	}

	return paths
}

func componentInjectionDir(homeDir, workspaceDir string, adapter agents.Adapter) string {
	if adapter.Agent() == model.AgentOpenClaw && strings.TrimSpace(workspaceDir) != "" {
		return workspaceDir
	}
	return homeDir
}

type openClawWorkspaceConfig struct {
	Agents struct {
		Defaults struct {
			Workspace string `json:"workspace"`
		} `json:"defaults"`
	} `json:"agents"`
}

func resolveOpenClawWorkspaceDir(homeDir, fallback string, agentIDs []model.AgentID) string {
	if !containsAgent(agentIDs, model.AgentOpenClaw) {
		return fallback
	}

	configPath := filepath.Join(homeDir, ".openclaw", "openclaw.json")
	content, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		return fallback
	}

	var config openClawWorkspaceConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fallback
	}

	workspace := strings.TrimSpace(config.Agents.Defaults.Workspace)
	if workspace == "" {
		return fallback
	}
	if filepath.IsAbs(workspace) {
		return filepath.Clean(workspace)
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return filepath.Clean(workspace)
	}
	return abs
}

func componentPathDir(homeDir, workspaceDir string, adapter agents.Adapter, component model.ComponentID) string {
	switch component {
	case model.ComponentDxrkMemory, model.ComponentSDD, model.ComponentPersona:
		return componentInjectionDir(homeDir, workspaceDir, adapter)
	default:
		return homeDir
	}
}

func sddSubAgentPaths(homeDir string, adapter agents.Adapter) []string {
	if !adapter.SupportsSubAgents() {
		return nil
	}

	entries, err := assets.FS.ReadDir(adapter.EmbeddedSubAgentsDir())
	if err != nil {
		return nil
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(adapter.SubAgentsDir(homeDir), entry.Name()))
	}

	return paths
}

func runPostApplyVerification(homeDir, workspaceDir string, selection model.Selection, resolved planner.ResolvedPlan) verify.Report {
	checks := make([]verify.Check, 0)
	adapters := resolveAdapters(resolved.Agents)

	seenPath := make(map[string]struct{})
	var uniqueFilePaths []string
	for _, component := range resolved.OrderedComponents {
		for _, path := range componentPathsWithWorkspace(homeDir, workspaceDir, selection, adapters, component) {
			if path == "" {
				continue
			}
			if _, dup := seenPath[path]; dup {
				continue
			}
			seenPath[path] = struct{}{}
			uniqueFilePaths = append(uniqueFilePaths, path)
		}
	}

	for _, currentPath := range uniqueFilePaths {
		path := currentPath
		checks = append(checks, verify.Check{
			ID:          "verify:file:" + path,
			Description: "required file exists",
			Run: func(context.Context) error {
				if _, err := os.Stat(path); err != nil {
					return err
				}
				return nil
			},
		})
	}

	if hasComponent(resolved.OrderedComponents, model.ComponentDxrkMemory) {
		checks = append(checks, dxrkMemoryHealthChecks()...)
	}
	checks = append(checks, antigravityCollisionCheck(resolved.Agents)...)

	return verify.BuildReport(verify.RunChecks(context.Background(), checks))
}

func hasComponent(components []model.ComponentID, target model.ComponentID) bool {
	for _, c := range components {
		if c == target {
			return true
		}
	}
	return false
}

func containsAgent(agentIDs []model.AgentID, target model.AgentID) bool {
	for _, agent := range agentIDs {
		if agent == target {
			return true
		}
	}
	return false
}

func dxrkMemoryHealthChecks() []verify.Check {
	return []verify.Check{
		{
			ID:          "verify:dxrk-memory:binary",
			Description: "dxrk-memory binary on PATH (restart shell if missing)",
			Soft:        true,
			Run: func(context.Context) error {
				if err := dxrkmemory.VerifyInstalled(); err != nil {
					return fmt.Errorf("%w\nIf dxrk-memory was installed via `go install`, add it to PATH:\n  %s", err, dxrkMemoryPathGuidance(os.Getenv("SHELL")))
				}
				return nil
			},
		},
		{
			ID:          "verify:dxrk-memory:version",
			Description: "dxrk-memory version returns valid output",
			Soft:        true,
			Run: func(context.Context) error {
				if err := dxrkmemory.VerifyInstalled(); err != nil {
					// Binary not on PATH — skip version check gracefully.
					return err
				}
				_, err := dxrkmemory.VerifyVersion()
				return err
			},
		},
	}
}

// antigravityCollisionCheck returns a soft verify check that warns the user
// when both Antigravity and Gemini CLI are selected. Both agents write to
// ~/.gemini/GEMINI.md — content is merged (not overwritten) but the user
// should be aware.
func antigravityCollisionCheck(agentIDs []model.AgentID) []verify.Check {
	hasAntigravity := false
	hasGemini := false
	for _, id := range agentIDs {
		if id == model.AgentAntigravity {
			hasAntigravity = true
		}
		if id == model.AgentGeminiCLI {
			hasGemini = true
		}
	}
	if !hasAntigravity || !hasGemini {
		return nil
	}
	return []verify.Check{
		{
			ID:          "verify:antigravity:rules-collision",
			Description: "Antigravity and Gemini CLI share ~/.gemini/GEMINI.md",
			Soft:        true,
			Run: func(context.Context) error {
				return fmt.Errorf(
					"both Antigravity and Gemini CLI write rules to ~/.gemini/GEMINI.md: " +
						"content is merged, not overwritten — rules from both agents coexist in the same file" +
						"; this is expected behavior, no action required unless you want to separate them manually")
			},
		},
	}
}

func dxrkMemoryPathGuidance(shellPath string) string {
	binDir := goInstallBinDir()
	if strings.Contains(shellPath, "fish") {
		return fmt.Sprintf("set -Ux fish_user_paths %s $fish_user_paths", binDir)
	}
	if strings.Contains(shellPath, "zsh") {
		return fmt.Sprintf("echo 'export PATH=\"%s:$PATH\"' >> ~/.zshrc && source ~/.zshrc", binDir)
	}
	if strings.Contains(shellPath, "bash") {
		return fmt.Sprintf("echo 'export PATH=\"%s:$PATH\"' >> ~/.bashrc && source ~/.bashrc", binDir)
	}
	return fmt.Sprintf("Add %s to your shell PATH and restart the terminal.", binDir)
}

// checkDependenciesStep verifies that required system dependencies are present.
// It logs warnings for missing optional deps but only fails if required deps are missing.
type checkDependenciesStep struct {
	id        string
	profile   system.PlatformProfile
	homeDir   string
	selection model.Selection
}

func (s checkDependenciesStep) ID() string {
	return s.id
}

func (s checkDependenciesStep) Run() error {
	// Run detection but do NOT write to stdout/stderr — this step runs
	// inside the Bubble Tea alternate screen in TUI mode, so any raw
	// output corrupts the display (see issue #2). Missing deps are
	// surfaced on the TUI complete screen and by the actual install steps
	// failing with real error messages.
	_ = system.DetectDependencies(context.Background(), s.profile)
	for _, agent := range s.selection.Agents {
		adapter, err := agents.NewAdapter(agent)
		if err != nil {
			return fmt.Errorf("create adapter for %q: %w", agent, err)
		}

		if !adapter.SupportsAutoInstall() {
			continue
		}

		if s.homeDir != "" {
			installed, _, _, _, err := adapter.Detect(context.Background(), s.homeDir)
			if err != nil {
				return fmt.Errorf("detect agent %q: %w", agent, err)
			}
			if installed {
				continue
			}
		}

		if err := installcmd.ValidateAgentInstallPreflight(s.profile, agent); err != nil {
			return fmt.Errorf("preflight for agent %q: %w", agent, err)
		}
	}
	return nil
}

type noopStep struct {
	id string
}

func (s noopStep) ID() string {
	return s.id
}

func (s noopStep) Run() error {
	return nil
}

// claudeAliasesToStrings converts a typed ClaudeModelAlias map to plain strings
// for JSON serialisation in state.json.
func claudeAliasesToStrings(m map[string]model.ClaudeModelAlias) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		// Claude Code owns the main session/orchestrator model; do not persist it
		// as a Dxrk AI model assignment.
		if k == "orchestrator" {
			continue
		}
		out[k] = string(v)
	}
	return out
}

// modelAssignmentsToState converts model.ModelAssignment maps to the
// state-serialisable form.
func modelAssignmentsToState(m map[string]model.ModelAssignment) map[string]state.ModelAssignmentState {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]state.ModelAssignmentState, len(m))
	for k, v := range m {
		out[k] = state.ModelAssignmentState{ProviderID: v.ProviderID, ModelID: v.ModelID, Effort: v.Effort}
	}
	return out
}
