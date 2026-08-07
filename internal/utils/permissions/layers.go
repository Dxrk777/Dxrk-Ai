// SPDX-License-Identifier: MIT
package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// ---- Layer Definitions ----

// Layer represents a permission source layer with evaluation priority.
type Layer int

const (
	// LayerDefault is the built-in defaults (lowest priority).
	LayerDefault Layer = iota
	// LayerSession is runtime decisions (cached).
	LayerSession
	// LayerUser is user preferences in config.
	LayerUser
	// LayerProject is .dxrk/policies/ directory.
	LayerProject
	// LayerOrganization is org-wide policies (highest priority).
	LayerOrganization
)

func (l Layer) String() string {
	switch l {
	case LayerDefault:
		return "default"
	case LayerSession:
		return "session"
	case LayerUser:
		return "user"
	case LayerProject:
		return strconst.StrProject
	case LayerOrganization:
		return "organization"
	default:
		return strconst.StrUnknown
	}
}

// Priority returns the numeric priority (higher = evaluated first).
func (l Layer) Priority() int {
	return int(l)
}

// ---- Layered Policy ----

// LayerPolicy associates rules with a specific layer.
type LayerPolicy struct {
	Layer  Layer
	Policy *Policy
}

// LayeredPolicy manages rules across multiple ordered layers.
type LayeredPolicy struct {
	layers   []LayerPolicy
	fallback *Policy
}

// NewLayeredPolicy creates a layered policy with default configuration.
func NewLayeredPolicy() *LayeredPolicy {
	return &LayeredPolicy{
		layers: []LayerPolicy{
			{Layer: LayerDefault, Policy: defaultPolicy()},
		},
		fallback: defaultPolicy(),
	}
}

func defaultPolicy() *Policy {
	return &Policy{
		Name:          "default",
		Version:       "1.0",
		DefaultAction: Ask,
		Strategy:      FirstMatch,
		Rules: []Rule{
			{ID: "default-read", Subject: "*", Resource: "Read", Action: Allow, Priority: 0},
			{ID: "default-glob", Subject: "*", Resource: "Glob", Action: Allow, Priority: 0},
			{ID: "default-grep", Subject: "*", Resource: "Grep", Action: Allow, Priority: 0},
			{ID: "default-ls", Subject: "*", Resource: "LS", Action: Allow, Priority: 0},
			{ID: "default-bash", Subject: "*", Resource: "Bash", Action: Ask, Priority: 0},
		},
	}
}

// Evaluate evaluates all layers from highest to lowest priority.
// Returns the first definitive action (Allow/Deny). If all layers return Ask,
// Ask is returned. The rule and layer name that produced the result are
// included.
func (lp *LayeredPolicy) Evaluate(ctx *EvalContext) (Action, *Rule, string, error) {
	sorted := lp.sortedLayers()

	for _, lpEntry := range sorted {
		engine := NewPolicyEngine(*lpEntry.Policy)
		action, rule, err := engine.Evaluate(ctx)
		if err != nil {
			return Ask, nil, lpEntry.Layer.String(), fmt.Errorf("evaluate layer %s: %w", lpEntry.Layer, err)
		}
		if action == Deny {
			return Deny, rule, lpEntry.Layer.String(), nil
		}
		if action == Allow {
			return Allow, rule, lpEntry.Layer.String(), nil
		}
	}

	return Ask, nil, "none", nil
}

func (lp *LayeredPolicy) sortedLayers() []LayerPolicy {
	sorted := make([]LayerPolicy, len(lp.layers))
	copy(sorted, lp.layers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Layer.Priority() > sorted[j].Layer.Priority()
	})
	return sorted
}

// AddLayer adds or replaces a layer.
func (lp *LayeredPolicy) AddLayer(layer Layer, p *Policy) {
	for i, entry := range lp.layers {
		if entry.Layer == layer {
			lp.layers[i].Policy = p
			return
		}
	}
	lp.layers = append(lp.layers, LayerPolicy{Layer: layer, Policy: p})
}

// RemoveLayer removes a layer by kind.
func (lp *LayeredPolicy) RemoveLayer(layer Layer) {
	for i, entry := range lp.layers {
		if entry.Layer == layer {
			lp.layers = append(lp.layers[:i], lp.layers[i+1:]...)
			return
		}
	}
}

// Layers returns all configured layers.
func (lp *LayeredPolicy) Layers() []LayerPolicy {
	out := make([]LayerPolicy, len(lp.layers))
	copy(out, lp.layers)
	return out
}

// ---- Layer Merge ----

// LayerMerge merges rules from base and override, with override taking precedence
// on duplicate IDs.
func LayerMerge(base, override []Rule) []Rule {
	byID := make(map[string]Rule, len(base)+len(override))
	for _, r := range base {
		byID[r.ID] = r
	}
	for _, r := range override {
		byID[r.ID] = r
	}
	result := make([]Rule, 0, len(byID))
	for _, r := range byID {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// ---- File Loading ----

// LoadProjectPolicy loads a policy from a .dxrk/policies/ directory.
// It reads all JSON files in the directory and merges them.
func LoadProjectPolicy(dir string) (*Policy, error) {
	policyDir := filepath.Join(dir, ".dxrk", "policies")
	entries, err := os.ReadDir(policyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Policy{
				Name:          strconst.StrProject,
				DefaultAction: Ask,
				Strategy:      FirstMatch,
			}, nil
		}
		return nil, fmt.Errorf("read project policies dir: %w", err)
	}

	merged := &Policy{
		Name:          strconst.StrProject,
		Version:       "1.0",
		DefaultAction: Ask,
		Strategy:      FirstMatch,
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(policyDir, entry.Name())
		p, err := LoadPolicyFile(path)
		if err != nil {
			return nil, fmt.Errorf("load project policy %q: %w", path, err)
		}
		merged.Rules = append(merged.Rules, p.Rules...)
		if p.DefaultAction == Deny {
			merged.DefaultAction = Deny
		}
	}

	return merged, nil
}

// LoadUserPolicy loads a policy from the user's config directory.
func LoadUserPolicy(configDir string) (*Policy, error) {
	path := filepath.Join(configDir, "permissions", "policy.json")
	p, err := LoadPolicyFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Policy{
				Name:          "user",
				DefaultAction: Ask,
				Strategy:      FirstMatch,
			}, nil
		}
		return nil, fmt.Errorf("load user policy: %w", err)
	}
	return p, nil
}

// ---- Serialization ----

// MarshalLayeredPolicyJSON serializes a layered policy to JSON.
func MarshalLayeredPolicyJSON(lp *LayeredPolicy) ([]byte, error) {
	return json.MarshalIndent(lp.layers, "", "  ")
}
