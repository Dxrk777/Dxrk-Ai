// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/spf13/cobra"
)

const planFileName = "plan.json"

// Plan represents an implementation plan.
type Plan struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Phases      []Phase   `json:"phases"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Phase represents a phase in an implementation plan.
type Phase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Tasks       []Task `json:"tasks"`
}

// Task represents a task within a phase.
type Task struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee,omitempty"`
}

// PlanConfig is the top-level plan configuration.
type PlanConfig struct {
	Plans []Plan `json:"plans"`
}

func planConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "dxrk", planFileName), nil
}

func loadPlanConfig() (*PlanConfig, error) {
	path, err := planConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return &PlanConfig{}, nil
		}
		return nil, fmt.Errorf("read plan config: %w", err)
	}

	var cfg PlanConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse plan config: %w", err)
	}
	return &cfg, nil
}

func savePlanConfig(cfg *PlanConfig) error {
	path, err := planConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o600) //nolint:gosec
}

// RegisterPlanCommand adds the plan subcommand to the given root command.
func RegisterPlanCommand(root *cobra.Command) {
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Create and manage implementation plans",
		Long:  `Create, list, show, update, and delete implementation plans.`,
	}

	planCmd.AddCommand(
		newPlanCreateCmd(),
		newPlanListCmd(),
		newPlanShowCmd(),
		newPlanUpdateCmd(),
		newPlanDeleteCmd(),
	)

	root.AddCommand(planCmd)
}

func newPlanCreateCmd() *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new implementation plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadPlanConfig()
			if err != nil {
				return err
			}

			for _, p := range cfg.Plans {
				if p.Name == name {
					return fmt.Errorf("plan %q already exists", name)
				}
			}

			now := time.Now()
			plan := Plan{
				Name:        name,
				Description: description,
				Status:      "draft",
				Phases: []Phase{
					{
						Name:        "Phase 1: Planning",
						Description: "Initial planning and research",
						Status:      strconst.StrPending,
						Tasks: []Task{
							{Name: "Define requirements", Status: strconst.StrPending},
							{Name: "Identify dependencies", Status: strconst.StrPending},
						},
					},
					{
						Name:        "Phase 2: Implementation",
						Description: "Core implementation work",
						Status:      strconst.StrPending,
						Tasks: []Task{
							{Name: "Implement core logic", Status: strconst.StrPending},
							{Name: "Write tests", Status: strconst.StrPending},
						},
					},
					{
						Name:        "Phase 3: Review",
						Description: "Review and validation",
						Status:      strconst.StrPending,
						Tasks: []Task{
							{Name: "Code review", Status: strconst.StrPending},
							{Name: "Integration testing", Status: strconst.StrPending},
						},
					},
				},
				CreatedAt: now,
				UpdatedAt: now,
			}

			cfg.Plans = append(cfg.Plans, plan)
			if err := savePlanConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Plan %q created.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, strconst.StrDescription, "d", "", "Plan description")
	return cmd
}

func newPlanListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all implementation plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadPlanConfig()
			if err != nil {
				return err
			}

			if len(cfg.Plans) == 0 {
				fmt.Println("No plans found. Create one with: dxrk plan create <name>")
				return nil
			}

			for _, p := range cfg.Plans {
				fmt.Printf("  %s [%s] - %s\n", p.Name, p.Status, p.Description)
			}
			return nil
		},
	}
}

func newPlanShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show plan details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadPlanConfig()
			if err != nil {
				return err
			}

			for _, p := range cfg.Plans {
				if p.Name == name {
					fmt.Printf("Name:        %s\n", p.Name)
					fmt.Printf("Description: %s\n", p.Description)
					fmt.Printf("Status:      %s\n", p.Status)
					fmt.Printf("Created:     %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
					fmt.Printf("Updated:     %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
					fmt.Println("\nPhases:")
					for _, phase := range p.Phases {
						fmt.Printf("  %s [%s]\n", phase.Name, phase.Status)
						fmt.Printf("    %s\n", phase.Description)
						for _, task := range phase.Tasks {
							assignee := ""
							if task.Assignee != "" {
								assignee = " (" + task.Assignee + ")"
							}
							fmt.Printf("    - %s [%s]%s\n", task.Name, task.Status, assignee)
						}
					}
					return nil
				}
			}

			return fmt.Errorf("plan %q not found", name)
		},
	}
}

func newPlanUpdateCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a plan's status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadPlanConfig()
			if err != nil {
				return err
			}

			found := false
			for i := range cfg.Plans {
				if cfg.Plans[i].Name == name {
					if status != "" {
						cfg.Plans[i].Status = status
					}
					cfg.Plans[i].UpdatedAt = time.Now()
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("plan %q not found", name)
			}

			if err := savePlanConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Plan %q updated.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, strconst.StrStatus, "s", "", "New status (draft, active, completed, archived)")
	return cmd
}

func newPlanDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadPlanConfig()
			if err != nil {
				return err
			}

			found := false
			var filtered []Plan
			for _, p := range cfg.Plans {
				if p.Name == name {
					found = true
					continue
				}
				filtered = append(filtered, p)
			}

			if !found {
				return fmt.Errorf("plan %q not found", name)
			}

			cfg.Plans = filtered
			if err := savePlanConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Plan %q deleted.\n", name)
			return nil
		},
	}
}
