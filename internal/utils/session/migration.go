package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// MigrationFunc transforms session JSON from one version to the next.
type MigrationFunc func(data []byte) ([]byte, error)

type migrationEntry struct {
	from int
	to   int
	fn   MigrationFunc
}

var (
	migrationsMu    sync.Mutex
	migrations      []migrationEntry
	migrationsBuilt bool
)

func init() { buildMigrations() }

// RegisterMigration adds a migration path. Duplicate paths are ignored.
func RegisterMigration(from, to int, fn MigrationFunc) {
	migrationsMu.Lock()
	defer migrationsMu.Unlock()
	for _, m := range migrations {
		if m.from == from && m.to == to {
			return
		}
	}
	migrations = append(migrations, migrationEntry{from: from, to: to, fn: fn})
}

// MigrateSession upgrades session data from one version to another.
func MigrateSession(data []byte, fromVersion, toVersion int) ([]byte, error) {
	if fromVersion == toVersion {
		return data, nil
	}
	if fromVersion > toVersion {
		return nil, fmt.Errorf("downgrade migrations not supported: %d -> %d", fromVersion, toVersion)
	}

	current := fromVersion
	currentData := data
	for current < toVersion {
		fn := findMigration(current, current+1)
		if fn == nil {
			return nil, fmt.Errorf("no migration from version %d to %d", current, current+1)
		}
		var err error
		currentData, err = fn(currentData)
		if err != nil {
			return nil, fmt.Errorf("migration %d -> %d failed: %w", current, current+1, err)
		}
		var probe struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(currentData, &probe); err == nil && probe.Version > current {
			current = probe.Version
		} else {
			current++
		}
	}
	return currentData, nil
}

func findMigration(from, to int) MigrationFunc {
	migrationsMu.Lock()
	defer migrationsMu.Unlock()
	for _, m := range migrations {
		if m.from == from && m.to == to {
			return m.fn
		}
	}
	return nil
}

func buildMigrations() {
	migrationsMu.Lock()
	defer migrationsMu.Unlock()
	if migrationsBuilt {
		return
	}
	migrationsBuilt = true

	// v1 → v2: add version field, convert int status to string, ensure messages array.
	RegisterMigration(1, 2, func(data []byte) ([]byte, error) {
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("unmarshal v1: %w", err)
		}
		raw[strconst.StrVersion] = 2
		if _, ok := raw["messages"]; !ok {
			raw["messages"] = []any{}
		}
		if status, ok := raw[strconst.StrStatus].(float64); ok {
			names := map[int]string{0: strconst.StrActive, 1: "paused", 2: strconst.StrCompleted, 3: "archived", 4: "expired"}
			if name, ok := names[int(status)]; ok {
				raw[strconst.StrStatus] = name
			} else {
				raw[strconst.StrStatus] = strconst.StrActive
			}
		}
		return json.MarshalIndent(raw, "", "  ")
	})
}

// ListMigrations returns the registered migration paths sorted.
func ListMigrations() []string {
	migrationsMu.Lock()
	defer migrationsMu.Unlock()
	paths := make([]string, 0, len(migrations))
	for _, m := range migrations {
		paths = append(paths, fmt.Sprintf("%d -> %d", m.from, m.to))
	}
	sort.Strings(paths)
	return paths
}

// HasMigration returns true if a migration path exists.
func HasMigration(from, to int) bool { return findMigration(from, to) != nil }

// DetectVersion reads the version field from raw JSON.
func DetectVersion(data []byte) (int, error) {
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, fmt.Errorf("detect version: %w", err)
	}
	return probe.Version, nil
}

// MigrateToCurrent migrates data to the current package version.
func MigrateToCurrent(data []byte) ([]byte, error) {
	v, err := DetectVersion(data)
	if err != nil {
		return nil, err
	}
	return MigrateSession(data, v, CurrentVersion)
}

func init() { _ = strings.TrimSpace } // keep strings import used
