// SPDX-License-Identifier: MIT
package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/assets"
	"github.com/Dxrk777/Dxrk/internal/components/filemerge"
	"github.com/Dxrk777/Dxrk/internal/model"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

// bootstrapper is an optional adapter capability: if an adapter implements
// this interface, any injector that writes Jinja modules will first ensure
// the base template (entry point) exists.
type bootstrapper interface {
	BootstrapTemplate(homeDir string) error
}

// outputStyleOverlayJSON is the settings.json overlay to enable the Dxrk output style.
var outputStyleOverlayJSON = []byte("{\n  \"outputStyle\": \"Dxrk\"\n}\n")

// openCodeAgentOverlayJSON defines the Tab-switchable persona agent for OpenCode.
// SDD is installed separately by the SDD component as "dxrk-orchestrator";
// persona injection must not create legacy SDD conductor keys.
var openCodeAgentOverlayJSON = []byte("{\n  \"agent\": {\n    \"dxrk\": {\n      \"mode\": \"primary\",\n      \"description\": \"Senior Architect mentor - helpful first, challenging when it matters\",\n      \"prompt\": \"{file:./AGENTS.md}\",\n      \"tools\": {\n        \"write\": true,\n        \"edit\": true\n      }\n    }\n  }\n}\n")

// Inject performs a full persona injection: the marker-bound markdown block,
// the OpenCode/Kilocode `dxrk` agent definition in settings JSON, AND
// the Claude Code output-style overlay. Used by `dxrk install`.
func Inject(homeDir string, adapter agents.Adapter, persona model.PersonaID) (InjectionResult, error) {
	return injectInternal(homeDir, adapter, persona, false)
}

// InjectForSync regenerates the persona assets that `dxrk sync` is
// allowed to touch. It writes:
//   - The marker-bound persona block in the agent's prompt file (markdown).
//   - The Dxrk output-style file + outputStyle settings overlay (Claude
//     Code only — no conflict with other components).
//
// It deliberately skips the OpenCode/Kilocode `dxrk` agent definition in
// opencode.json/kilocode.json: that JSON merge shares the "agent" key with
// SDD's dxrk-orchestrator overlay, so running both in the same sync clobbers
// each other's entries and breaks idempotency. That overlay remains an
// install-only concern.
func InjectForSync(homeDir string, adapter agents.Adapter, persona model.PersonaID) (InjectionResult, error) {
	return injectInternal(homeDir, adapter, persona, true)
}

// syncManaged is the internal flag previously called `markdownOnly`.
// When true the OpenCode/Kilocode agent overlay is skipped (see InjectForSync).
func injectInternal(homeDir string, adapter agents.Adapter, persona model.PersonaID, syncManaged bool) (InjectionResult, error) {
	if !adapter.SupportsSystemPrompt() {
		return InjectionResult{}, nil
	}
	if err := validateOpenClawWorkspacePath(homeDir, adapter); err != nil {
		return InjectionResult{}, err
	}

	// Custom persona does nothing — user keeps their own config.
	if persona == model.PersonaCustom {
		return InjectionResult{}, nil
	}

	files := make([]string, 0, 3)
	changed := false

	content := personaContent(adapter.Agent(), persona)
	if content == "" {
		return InjectionResult{}, nil
	}

	// 1. Inject persona content based on system prompt strategy.
	if adapter.Agent() == model.AgentOpenClaw {
		return injectOpenClawSoulPersona(homeDir, content)
	}

	switch adapter.SystemPromptStrategy() {
	case model.StrategyMarkdownSections:
		promptPath := adapter.SystemPromptFile(homeDir)
		existing, err := readFileOrEmpty(promptPath)
		if err != nil {
			return InjectionResult{}, err
		}

		// Auto-heal: strip any legacy free-text Dxrk persona block that was
		// written before the marker-based injection system existed. This is safe
		// for StrategyMarkdownSections because InjectMarkdownSection preserves
		// all existing marker sections — only the unmarked free-text preamble is
		// removed, and StripLegacyPersonaBlock requires ALL three fingerprints
		// to be present in the pre-marker zone before stripping.
		healed := filemerge.StripLegacyPersonaBlock(existing)

		// Also strip legacy Agent Teams Lite block (standalone ATL installer leftover).
		healed = filemerge.StripLegacyATLBlock(healed)

		updated := filemerge.InjectMarkdownSection(healed, "persona", content)

		writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || writeResult.Changed
		files = append(files, promptPath)

	case model.StrategyFileReplace:
		promptPath := adapter.SystemPromptFile(homeDir)

		if adapter.Agent() == model.AgentOpenCode {
			existing, err := readFileOrEmpty(promptPath)
			if err != nil {
				return InjectionResult{}, err
			}

			healed := existing

			// Only strip legacy persona when a managed persona section already
			// exists — that is the only strong proof the pre-marker content is
			// stale installer output, not user-authored content.
			if shouldStripManagedLegacyPersona(existing) {
				healed = filemerge.StripLegacyPersonaBlock(existing)
			} else if isExactLegacyPersonaAsset(existing) {
				// The file is byte-for-byte the old installer asset with no
				// markers. Safe to replace entirely — no user content to lose.
				healed = ""
			}

			healed = filemerge.StripLegacyATLBlock(healed)
			updated := filemerge.InjectMarkdownSection(healed, "persona", content)

			writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || writeResult.Changed
			files = append(files, promptPath)
			break
		}

		// For non-Dxrk personas (e.g. neutral), the content is just a short
		// one-liner. Writing ONLY that content would destroy any SDD/dxrk-memory
		// sections that are injected later in the pipeline. Instead, we write the
		// persona content as the base and let subsequent inject steps (SDD, dxrk-memory)
		// append their sections. For Dxrk, the content is the full persona
		// asset which is safe to write as-is.
		//
		// If the file already exists and has managed sections (SDD, dxrk-memory), we
		// must preserve them — replace only the persona portion at the top.
		existing, readErr := readFileOrEmpty(promptPath)
		if readErr != nil {
			return InjectionResult{}, readErr
		}

		if preserved, ok := preserveManagedSections(existing, content, persona); ok {
			writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(preserved), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || writeResult.Changed
			files = append(files, promptPath)
			break
		}

		writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(content), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || writeResult.Changed
		files = append(files, promptPath)

	case model.StrategyInstructionsFile:
		promptPath := adapter.SystemPromptFile(homeDir)

		// Auto-heal: remove any stale Dxrk persona content left at the
		// old VSCode path (~/.github/copilot-instructions.md) that was written
		// by an older installer version.  VS Code still reads that path for
		// global instructions, so the two files would conflict.
		if cleaned, cleanErr := cleanLegacyVSCodePersona(homeDir); cleanErr == nil && cleaned {
			changed = true
		}

		// For non-Dxrk personas, preserve managed sections (same logic
		// as StrategyFileReplace above).
		existing, readErr := readFileOrEmpty(promptPath)
		if readErr != nil {
			return InjectionResult{}, readErr
		}

		if preserved, ok := preserveManagedSections(existing, wrapInstructionsFile(content), persona); ok {
			writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(preserved), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || writeResult.Changed
			files = append(files, promptPath)
			break
		}

		// Write the new instructions file (with YAML frontmatter) to the current path.
		// WriteFileAtomic compares bytes, so it is naturally idempotent: it rewrites
		// whenever the on-disk content differs from instructionsContent, which covers
		// the case where an older install wrote persona content without frontmatter.
		instructionsContent := wrapInstructionsFile(content)
		writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(instructionsContent), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || writeResult.Changed
		files = append(files, promptPath)

	case model.StrategySteeringFile:
		promptPath := adapter.SystemPromptFile(homeDir)

		existing, readErr := readFileOrEmpty(promptPath)
		if readErr != nil {
			return InjectionResult{}, readErr
		}

		var steeringContent string
		if preserved, ok := preserveManagedSections(existing, wrapSteeringFile(content), persona); ok {
			steeringContent = preserved
		} else {
			steeringContent = wrapSteeringFile(content)
		}

		if err := os.MkdirAll(filepath.Dir(promptPath), 0o750); err != nil {
			return InjectionResult{}, err
		}
		writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(steeringContent), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || writeResult.Changed
		files = append(files, promptPath)

	case model.StrategyAppendToFile:
		promptPath := adapter.SystemPromptFile(homeDir)

		// Read existing content if file exists
		existing, err := readFileOrEmpty(promptPath)
		if err != nil {
			return InjectionResult{}, err
		}

		// Idempotency: skip if persona content is already present in the file.
		if strings.Contains(existing, strings.TrimSpace(content)) {
			return InjectionResult{Files: []string{promptPath}}, nil
		}

		// Do a real append: preserve existing content + add new content
		updated := existing
		if len(updated) > 0 && !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
		if len(updated) > 0 {
			updated += "\n"
		}
		updated += content

		writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || writeResult.Changed
		files = append(files, promptPath)

	case model.StrategyJinjaModules:
		// Ensure the base template exists for Jinja-based agents.
		if bs, ok := adapter.(bootstrapper); ok {
			if err := bs.BootstrapTemplate(homeDir); err != nil {
				return InjectionResult{}, fmt.Errorf("bootstrap template: %w", err)
			}
			files = append(files, adapter.SystemPromptFile(homeDir))
			files = append(files, adapter.SettingsPath(homeDir))
		}

		// Write separate Jinja include modules for Kimi (and any future agents that
		// use this strategy). Each module corresponds to one {% include "…" %} in
		// the static KIMI.md template that the bootstrapper above ensures exists.
		configDir := adapter.GlobalConfigDir(homeDir)

		// Module 1: persona (raw content — no variables; those live in the template).
		personaPath := filepath.Join(configDir, "persona.md")
		wr1, err := filemerge.WriteFileAtomic(personaPath, []byte(content), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || wr1.Changed
		files = append(files, personaPath)

		// Module 2: output-style (Dxrk only; empty file for neutral keeps the
		// include harmless via "ignore missing" in the template).
		outputStyleContent := ""
		if persona == model.PersonaDxrk {
			outputStyleContent = assets.MustRead("kimi/output-style-dxrk.md")
		}
		outputStylePath := filepath.Join(configDir, "output-style.md")
		wr2, err := filemerge.WriteFileAtomic(outputStylePath, []byte(outputStyleContent), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || wr2.Changed
		files = append(files, outputStylePath)
	}

	// 2. OpenCode/Kilocode agent definitions — Tab-switchable agents in settings.
	// Skipped under syncManaged because this overlay shares the "agent" key in
	// opencode.json with SDD's dxrk-orchestrator overlay; running both in the
	// same sync (in either order) makes them clobber each other's entries and
	// breaks idempotency. Install handles this overlay once at install time.
	if !syncManaged && (adapter.Agent() == model.AgentOpenCode || adapter.Agent() == model.AgentKilocode) && persona != model.PersonaCustom {
		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath != "" {
			if persona == model.PersonaDxrk {
				agentResult, err := mergeJSONFile(settingsPath, openCodeAgentOverlayJSON)
				if err != nil {
					return InjectionResult{}, err
				}
				changed = changed || agentResult.Changed
				files = append(files, settingsPath)
			} else {
				// Non-dxrk: remove any residual agent.dxrk key left by a
				// previous dxrk install. Only the "dxrk" sub-key is removed
				// from within "agent" — other user-defined agents are preserved.
				removed, err := removeJSONNestedSubKey(settingsPath, "agent", "dxrk")
				if err != nil {
					return InjectionResult{}, fmt.Errorf("clean agent.dxrk from settings: %w", err)
				}
				if removed {
					changed = true
					files = append(files, settingsPath)
				}
			}
		}
	}

	// 3. Dxrk-only: write output style + merge into settings (if agent supports it).
	if persona == model.PersonaDxrk && adapter.Agent() != model.AgentOpenClaw && adapter.SupportsOutputStyles() {
		outputStyleDir := adapter.OutputStyleDir(homeDir)
		if outputStyleDir != "" {
			outputStylePath := outputStyleDir + "/dxrk.md"
			outputStyleContent := assets.MustRead("claude/output-style-dxrk.md")

			styleResult, err := filemerge.WriteFileAtomic(outputStylePath, []byte(outputStyleContent), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || styleResult.Changed
			files = append(files, outputStylePath)
		}

		// Merge "outputStyle": "Dxrk" into settings.
		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath != "" {
			settingsResult, err := mergeJSONFile(settingsPath, outputStyleOverlayJSON)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || settingsResult.Changed
			files = append(files, settingsPath)
		}
	}

	// 3b. Non-dxrk cleanup: remove residual Dxrk output-style artifacts
	// left by a previous install when the user switches away from the dxrk persona.
	if persona != model.PersonaDxrk && adapter.Agent() != model.AgentOpenClaw && adapter.SupportsOutputStyles() {
		outputStyleDir := adapter.OutputStyleDir(homeDir)
		if outputStyleDir != "" {
			outputStylePath := outputStyleDir + "/dxrk.md"
			styleRemoved, err := removeFileAtomic(outputStylePath)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("remove dxrk output style: %w", err)
			}
			if styleRemoved {
				changed = true
				files = append(files, outputStylePath)
			}
		}

		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath != "" {
			removed, err := removeJSONKeyIfValue(settingsPath, "outputStyle", "Dxrk")
			if err != nil {
				return InjectionResult{}, fmt.Errorf("clean outputStyle from settings: %w", err)
			}
			if removed {
				changed = true
				files = append(files, settingsPath)
			}
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

func validateOpenClawWorkspacePath(workspaceDir string, adapter agents.Adapter) error {
	if adapter.Agent() == model.AgentOpenClaw && strings.TrimSpace(workspaceDir) == "" {
		return fmt.Errorf("openclaw workspace path is required for workspace-first injection")
	}
	return nil
}

func injectOpenClawSoulPersona(workspaceDir, content string) (InjectionResult, error) {
	soulPath := filepath.Join(workspaceDir, "SOUL.md")
	existing, err := readFileOrEmpty(soulPath)
	if err != nil {
		return InjectionResult{}, err
	}

	healed := filemerge.StripLegacyPersonaBlock(existing)
	healed = filemerge.StripLegacyATLBlock(healed)
	updated := filemerge.InjectMarkdownSection(healed, "persona", content)

	writeResult, err := filemerge.WriteFileAtomic(soulPath, []byte(updated), 0o600)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{soulPath}}, nil
}

// shouldStripManagedLegacyPersona returns true ONLY when the existing file
// already contains a <!-- dxrk:persona --> section. That is the strongest
// evidence that the pre-marker persona content is stale legacy text written by
// an older installer, not user-authored content that happens to share headings.
//
// We intentionally do NOT trigger on ATL markers, dxrk-memory markers, sdd markers,
// or any other managed marker — their presence does not prove that the
// pre-marker content is installer-owned.
// isExactLegacyPersonaAsset returns true when the file content is an exact
// match of one of the known persona assets (dxrk or neutral). This handles
// the case where an old installer wrote the asset as the entire file with no
// markers — we can safely replace it because there is zero user content.
func isExactLegacyPersonaAsset(existing string) bool {
	trimmed := strings.TrimSpace(existing)
	if trimmed == "" {
		return false
	}
	for _, assetPath := range []string{
		"opencode/persona-dxrk.md",
		"generic/persona-dxrk.md",
		"generic/persona-neutral.md",
	} {
		asset := strings.TrimSpace(assets.MustRead(assetPath))
		if trimmed == asset {
			return true
		}
	}
	return false
}

func shouldStripManagedLegacyPersona(existing string) bool {
	return strings.Contains(existing, "<!-- dxrk:persona -->")
}

func personaContent(agent model.AgentID, persona model.PersonaID) string {
	switch persona {
	case model.PersonaNeutral:
		return assets.MustRead("generic/persona-neutral.md")
	case model.PersonaCustom:
		return ""
	default:
		// Dxrk persona — try agent-specific asset, then generic fallback.
		switch agent {
		case model.AgentClaudeCode:
			return assets.MustRead("claude/persona-dxrk.md")
		case model.AgentOpenCode, model.AgentKilocode:
			return assets.MustRead("opencode/persona-dxrk.md")
		case model.AgentKimi:
			return assets.MustRead("kimi/persona-dxrk.md")
		case model.AgentKiroIDE:
			// Kiro uses a steering-file based persona. The asset is identical to
			// generic today but kept separate so it can diverge independently.
			return assets.MustRead("kiro/persona-dxrk.md")
		default:
			// Generic persona includes Dxrk personality + skills table + SDD orchestrator.
			// Used by Gemini CLI, Cursor, VS Code Copilot, and any future agents.
			return assets.MustRead("generic/persona-dxrk.md")
		}
	}
}

func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	baseJSON, err := osReadFile(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	return filemerge.WriteFileAtomic(path, merged, 0o600)
}

var osReadFile = func(path string) ([]byte, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read json file %q: %w", path, err)
	}

	return content, nil
}

// preserveManagedSections checks whether the existing file content has
// dxrk managed sections (SDD orchestrator, dxrk-memory protocol, etc.) and
// returns new content that preserves those sections while replacing only the
// persona text before them. Returns ("", false) when no preservation is needed
// (empty file, Dxrk persona, or no managed markers found).
func preserveManagedSections(existing, newPersona string, persona model.PersonaID) (string, bool) {
	if existing == "" || persona == model.PersonaDxrk {
		return "", false
	}

	idx := strings.Index(existing, "<!-- dxrk:")
	if idx < 0 {
		return "", false
	}

	managedSuffix := existing[idx:]
	updated := newPersona
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if idx > 0 {
		// There was persona content before the markers — add a blank line separator.
		updated += "\n"
	}
	updated += managedSuffix

	return updated, true
}

func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(data), nil
}

func wrapInstructionsFile(content string) string {
	frontmatter := "---\n" +
		"name: Dxrk AI Persona\n" +
		"description: Teaching-oriented persona with SDD orchestration and DxrkMemory protocol\n" +
		"applyTo: \"**\"\n" +
		"---\n\n"

	return frontmatter + content
}

func wrapSteeringFile(content string) string {
	frontmatter := "---\n" +
		"inclusion: always\n" +
		"---\n\n"

	return frontmatter + content
}

// isLegacyUnwrappedPersona reports whether content is a Dxrk persona
// file written by an older installer version without YAML frontmatter.
// Requires ALL fingerprints to match (not just one) to reduce false positives.
// This is only used for legacy path cleanup (e.g. ~/.github/copilot-instructions.md)
// where the file is at a known old installer path — the combination of legacy
// path + all fingerprints is strong enough evidence of installer ownership.
func isLegacyUnwrappedPersona(content string) bool {
	if strings.HasPrefix(content, "---\n") {
		// Already has YAML frontmatter — not a legacy file.
		return false
	}
	// Require ALL fingerprints — a user is unlikely to have all of these
	// exact strings in a hand-written file at the old legacy path.
	personaFingerprints := []string{
		"## Personality",
		"Senior Architect",
	}
	for _, fp := range personaFingerprints {
		if !strings.Contains(content, fp) {
			return false
		}
	}
	return true
}

// legacyVSCodePersonaPaths returns the old VS Code persona file paths that may
// contain stale Dxrk persona content from older installer versions.
// These paths are no longer written by the current installer but may still
// be read by VS Code, causing conflicting instructions.
func legacyVSCodePersonaPaths(homeDir string) []string {
	return []string{
		// v1 path: wrote raw persona to ~/.github/copilot-instructions.md
		filepath.Join(homeDir, ".github", "copilot-instructions.md"),
	}
}

// removeFileAtomic removes path if it exists. Returns true when the file was
// present and successfully deleted, false when it did not exist. Any other
// OS-level error is returned as-is.
func removeFileAtomic(path string) (bool, error) {
	err := os.Remove(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// removeJSONKeyIfValue reads the JSON object at path, removes the top-level key
// only when its current string value equals wantValue, and writes the result
// back atomically. Returns true when the key was actually removed.
// If the file does not exist, the key is absent, or the value differs, it is
// a no-op and returns false.
func removeJSONKeyIfValue(path, key, wantValue string) (bool, error) {
	raw, err := osReadFile(path)
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}

	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		// Malformed settings — leave untouched to avoid data loss.
		return false, nil //nolint:nilerr
	}

	current, ok := root[key]
	if !ok {
		return false, nil
	}
	if current != wantValue {
		// User has a different value — do not touch it.
		return false, nil
	}

	delete(root, key)

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal settings after cleanup: %w", err)
	}
	encoded = append(encoded, '\n')

	if _, err := filemerge.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// removeJSONNestedSubKey reads the JSON object at path and removes subKey from
// within the top-level parentKey object. Only the named subKey is deleted —
// sibling keys inside parentKey are preserved. If the file does not exist, the
// parentKey is absent, or subKey is not present, it is a no-op and returns false.
func removeJSONNestedSubKey(path, parentKey, subKey string) (bool, error) {
	raw, err := osReadFile(path)
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}

	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return false, nil //nolint:nilerr
	}

	parent, ok := root[parentKey]
	if !ok {
		return false, nil
	}
	parentMap, ok := parent.(map[string]any)
	if !ok {
		return false, nil
	}
	if _, exists := parentMap[subKey]; !exists {
		return false, nil
	}

	delete(parentMap, subKey)
	if len(parentMap) == 0 {
		delete(root, parentKey)
	} else {
		root[parentKey] = parentMap
	}

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal settings after cleanup: %w", err)
	}
	encoded = append(encoded, '\n')

	if _, err := filemerge.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// cleanLegacyVSCodePersona removes Dxrk persona content from any old VS Code
// persona file paths that are no longer written by the current installer.
// Only files that contain clear Dxrk persona fingerprints are removed —
// files with user-written content are left untouched.
// Returns true if at least one file was cleaned.
func cleanLegacyVSCodePersona(homeDir string) (bool, error) {
	cleaned := false
	for _, oldPath := range legacyVSCodePersonaPaths(homeDir) {
		data, err := os.ReadFile(oldPath) //nolint:gosec
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cleaned, fmt.Errorf("read legacy vscode persona %q: %w", oldPath, err)
		}

		if !isLegacyUnwrappedPersona(string(data)) {
			// File exists but doesn't look like a Dxrk persona — leave it alone.
			continue
		}

		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			return cleaned, fmt.Errorf("remove legacy vscode persona %q: %w", oldPath, err)
		}
		cleaned = true
	}
	return cleaned, nil
}
