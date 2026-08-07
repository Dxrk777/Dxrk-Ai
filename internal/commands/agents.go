// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk/internal/coordinator"
	"github.com/spf13/cobra"
)

// RegisterAgentsCommand adds the agents subcommand to the given root command.
func RegisterAgentsCommand(root *cobra.Command) {
	agentsCmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage sub-agents",
		Long:  `List, create, inspect, and kill sub-agent instances.`,
	}

	agentsCmd.AddCommand(
		newAgentsListCmd(),
		newAgentsCreateCmd(),
		newAgentsStatusCmd(),
		newAgentsKillCmd(),
	)

	root.AddCommand(agentsCmd)
}

func newAgentsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sub-agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			coord := coordinator.NewCoordinator(coordinator.DefaultCoordinatorConfig())
			teams := coord.ListTeams()

			if len(teams) == 0 {
				fmt.Println("No agent teams found. Create one with: dxrk agents create <name> <member...>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TEAM\tMEMBERS\tSTATUS")
			for _, name := range teams {
				team, err := coord.GetTeam(name)
				if err != nil {
					continue
				}
				teamMu := team.Members
				members := ""
				statuses := ""
				for i, m := range teamMu {
					if i > 0 {
						members += ", "
						statuses += ", "
					}
					members += m.ID
					statuses += m.Status.String()
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, members, statuses)
			}
			return w.Flush()
		},
	}
}

func newAgentsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <team-name> <member...>",
		Short: "Create a new agent team",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamName := args[0]
			members := args[1:]

			coord := coordinator.NewCoordinator(coordinator.DefaultCoordinatorConfig())
			team, err := coord.CreateTeam(teamName, members)
			if err != nil {
				return fmt.Errorf("create team: %w", err)
			}

			fmt.Printf("Team %q created with members: %v\n", team.Name, team.MemberIDs())
			return nil
		},
	}
}

func newAgentsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [team-name]",
		Short: "Show agent status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			coord := coordinator.NewCoordinator(coordinator.DefaultCoordinatorConfig())

			if len(args) == 0 {
				// Show all teams
				teams := coord.ListTeams()
				if len(teams) == 0 {
					fmt.Println("No agent teams found.")
					return nil
				}
				for _, name := range teams {
					team, err := coord.GetTeam(name)
					if err != nil {
						continue
					}
					fmt.Printf("Team: %s\n", team.Name)
					for _, m := range team.Members {
						fmt.Printf("  %s: %s\n", m.ID, m.Status)
					}
				}
				return nil
			}

			team, err := coord.GetTeam(args[0])
			if err != nil {
				return fmt.Errorf("team %q not found: %w", args[0], err)
			}

			fmt.Printf("Team: %s\n", team.Name)
			for _, m := range team.Members {
				fmt.Printf("  %s: %s\n", m.ID, m.Status)
			}
			return nil
		},
	}
}

func newAgentsKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <team-name>",
		Short: "Kill an agent team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamName := args[0]
			coord := coordinator.NewCoordinator(coordinator.DefaultCoordinatorConfig())

			if err := coord.DeleteTeam(teamName); err != nil {
				return fmt.Errorf("kill team: %w", err)
			}

			fmt.Printf("Team %q killed.\n", teamName)
			return nil
		},
	}
}
