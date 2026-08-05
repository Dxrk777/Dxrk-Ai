// SPDX-License-Identifier: MIT
package commands

import (
	"github.com/spf13/cobra"
)

// Registry holds all registered session commands.
type Registry struct {
	Root *cobra.Command
}

// NewRegistry creates a new command registry under the given root command.
func NewRegistry(root *cobra.Command) *Registry {
	return &Registry{Root: root}
}

// AddCommand adds a cobra command to the root.
func (r *Registry) AddCommand(cmd *cobra.Command) {
	r.Root.AddCommand(cmd)
}
