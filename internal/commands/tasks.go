// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/task"
	"github.com/spf13/cobra"
)

// RegisterTasksCommand adds the tasks subcommand to the given root command.
func RegisterTasksCommand(root *cobra.Command) {
	tasksCmd := &cobra.Command{
		Use:   "tasks",
		Short: "Manage background tasks",
		Long:  `List, show, cancel, and inspect background tasks.`,
	}

	tasksCmd.AddCommand(
		newTasksListCmd(),
		newTasksShowCmd(),
		newTasksCancelCmd(),
		newTasksOutputCmd(),
	)

	root.AddCommand(tasksCmd)
}

func newTasksListCmd() *cobra.Command {
	var (
		status string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List background tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			queue := task.NewQueue()
			tasks := queue.List()

			var filtered []*task.Task
			for _, t := range tasks {
				if status != "" && t.Status.String() != status {
					continue
				}
				filtered = append(filtered, t)
			}

			switch output {
			case "json":
				type taskJSON struct {
					ID        string `json:"id"`
					Type      string `json:"type"`
					Status    string `json:"status"`
					Priority  int    `json:"priority"`
					CreatedAt string `json:"created_at"`
				}
				out := make([]taskJSON, 0, len(filtered))
				for _, t := range filtered {
					out = append(out, taskJSON{
						ID:        string(t.ID),
						Type:      string(t.Type),
						Status:    t.Status.String(),
						Priority:  t.Priority,
						CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			default:
				if len(filtered) == 0 {
					fmt.Println("No tasks found.")
					return nil
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tPRIORITY\tCREATED")
				for _, t := range filtered {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
						t.ID, t.Type, t.Status, t.Priority,
						t.CreatedAt.Format("2006-01-02 15:04:05"))
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVar(&status, strconst.StrStatus, "", "Filter by status (pending, running, completed, failed, cancelled)")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (text, json)")

	return cmd
}

func newTasksShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := task.TaskID(args[0])
			queue := task.NewQueue()
			t, ok := queue.Get(taskID)
			if !ok {
				return fmt.Errorf("task %q not found", taskID)
			}

			fmt.Printf("ID:        %s\n", t.ID)
			fmt.Printf("Type:      %s\n", t.Type)
			fmt.Printf("Status:    %s\n", t.Status)
			fmt.Printf("Priority:  %d\n", t.Priority)
			fmt.Printf("Created:   %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:   %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))

			if t.Err != nil {
				fmt.Printf("Error:     %v\n", t.Err)
			}

			if t.Result != nil {
				data, err := json.MarshalIndent(t.Result, "", "  ")
				if err == nil {
					fmt.Printf("Result:\n%s\n", data)
				}
			}

			return nil
		},
	}
}

func newTasksCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <task-id>",
		Short: "Cancel a background task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := task.TaskID(args[0])
			queue := task.NewQueue()
			t, ok := queue.Get(taskID)
			if !ok {
				return fmt.Errorf("task %q not found", taskID)
			}

			t.Cancel()
			fmt.Printf("Task %s cancelled.\n", taskID)
			return nil
		},
	}
}

func newTasksOutputCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "output <task-id>",
		Short: "Show task output/result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := task.TaskID(args[0])
			queue := task.NewQueue()
			t, ok := queue.Get(taskID)
			if !ok {
				return fmt.Errorf("task %q not found", taskID)
			}

			if t.Result == nil {
				fmt.Println("No output available (task may not have completed yet).")
				return nil
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(t.Result)
		},
	}
}
