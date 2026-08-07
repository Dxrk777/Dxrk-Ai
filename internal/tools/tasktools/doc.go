// Package tasktools provides background task management, output retrieval,
// and lifecycle control tools for the Dxrk agent system.
//
// It extends the coordinator package with persistent task tracking, structured
// output capture, scheduling, dependency resolution, and event-driven monitoring.
package tasktools

import "github.com/Dxrk777/Dxrk/internal/tools"

// RegisterAll registers all task management tools into the given registry.
func RegisterAll(reg *tools.Registry) error {
	for _, fn := range []func(*tools.Registry) error{
		registerTaskCreate,
		registerTaskUpdate,
		registerTaskOutput,
		registerTaskList,
		registerTaskCancel,
		registerTaskWait,
	} {
		if err := fn(reg); err != nil {
			return err
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }
