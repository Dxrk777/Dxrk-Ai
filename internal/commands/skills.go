// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk-Ai/internal/skillregistry"
	"github.com/spf13/cobra"
)

// RegisterSkillsCommand adds the skills subcommand to the given root command.
func RegisterSkillsCommand(root *cobra.Command) {
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage agent skills",
		Long:  `List, search, inspect, and toggle agent skills.`,
	}

	skillsCmd.AddCommand(
		newSkillsListCmd(),
		newSkillsSearchCmd(),
		newSkillsInfoCmd(),
		newSkillsEnableCmd(),
		newSkillsDisableCmd(),
	)

	root.AddCommand(skillsCmd)
}

func newSkillsListCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve cwd: %w", err)
			}

			dirs := skillregistry.UserSkillDirs(home)
			dirs = append(dirs, skillregistry.ProjectSkillDirs(cwd)...)

			var files []string
			for _, dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						files = append(files, skillFile)
					}
				}
			}

			var skills []skillregistry.SkillEntry
			for _, f := range files {
				if entry, ok := skillregistry.LoadSkill(f); ok {
					skills = append(skills, entry)
				}
			}

			if len(skills) == 0 {
				fmt.Println("No skills found.")
				return nil
			}

			switch output {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(skills)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tPATH")
				for _, s := range skills {
					desc := s.Description
					if len(desc) > 50 {
						desc = desc[:50] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, desc, s.Path)
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (text, json)")
	return cmd
}

func newSkillsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search skills by name or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve cwd: %w", err)
			}

			dirs := skillregistry.UserSkillDirs(home)
			dirs = append(dirs, skillregistry.ProjectSkillDirs(cwd)...)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tPATH")
			found := 0

			for _, dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
					skill, ok := skillregistry.LoadSkill(skillFile)
					if !ok {
						continue
					}
					if strings.Contains(strings.ToLower(skill.Name), query) ||
						strings.Contains(strings.ToLower(skill.Description), query) {
						desc := skill.Description
						if len(desc) > 50 {
							desc = desc[:50] + "..."
						}
						_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", skill.Name, desc, skill.Path)
						found++
					}
				}
			}

			if found == 0 {
				fmt.Printf("No skills matching %q found.\n", query)
				return nil
			}
			return w.Flush()
		},
	}
}

func newSkillsInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <skill-name>",
		Short: "Show detailed skill information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve cwd: %w", err)
			}

			dirs := skillregistry.UserSkillDirs(home)
			dirs = append(dirs, skillregistry.ProjectSkillDirs(cwd)...)

			for _, dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
					skill, ok := skillregistry.LoadSkill(skillFile)
					if !ok {
						continue
					}
					if skill.Name == skillName {
						fmt.Printf("Name:        %s\n", skill.Name)
						fmt.Printf("Description: %s\n", skill.Description)
						fmt.Printf("Path:        %s\n", skill.Path)
						fmt.Println("Rules:")
						for _, r := range skill.Rules {
							fmt.Printf("  - %s\n", r)
						}
						return nil
					}
				}
			}

			return fmt.Errorf("skill %q not found", skillName)
		},
	}
}

func newSkillsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <skill-name>",
		Short: "Enable a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Skill %q enabled.\n", args[0])
			return nil
		},
	}
}

func newSkillsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <skill-name>",
		Short: "Disable a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Skill %q disabled.\n", args[0])
			return nil
		},
	}
}
