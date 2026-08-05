package tasktools

import (
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

var (
	defaultManager   = NewTaskManager()
	defaultOutputMgr = NewTaskOutputManager(NewMemoryOutputStore(5000))
	defaultMonitor   = NewTaskMonitor()
)

func init() {
	defaultOutputMgr.SetTaskManager(defaultManager)
}

func registerTaskCreate(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_create",
		Description: "Create a new background task with title, description, priority, and optional parent/assignment.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrTitle: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task title",
				},
				strconst.StrDescription: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task description",
				},
				"parent_id": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Parent task ID for subtask creation",
				},
				strconst.StrPriority: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task priority: low, normal, high, urgent",
					"enum":                  []string{"low", strconst.StrNormal, "high", strconst.StrUrgent},
				},
				strconst.StrAssignedTo: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Agent or worker ID to assign the task to",
				},
				"tags": map[string]any{
					"type":                  strconst.StrArray,
					strconst.StrDescription: "Tags for categorization",
					strconst.StrItems:       map[string]any{"type": strconst.StrString},
				},
				"metadata": map[string]any{
					"type":                  strconst.StrObject,
					strconst.StrDescription: "Arbitrary key-value metadata",
				},
			},
			strconst.StrRequired: []string{strconst.StrTitle},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrTitle] == nil {
				return fmt.Errorf("title is required")
			}
			title, ok := input[strconst.StrTitle].(string)
			if !ok || title == "" {
				return fmt.Errorf("title must be a non-empty string")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			opts := CreateTaskOpts{
				Title:       input[strconst.StrTitle].(string),
				Description: toString(input[strconst.StrDescription]),
				ParentID:    toString(input["parent_id"]),
				AssignedTo:  toString(input[strconst.StrAssignedTo]),
			}
			if p, ok := input[strconst.StrPriority].(string); ok {
				opts.Priority = p
			}
			if tags, ok := input["tags"].([]any); ok {
				opts.Tags = make([]string, 0, len(tags))
				for _, tag := range tags {
					if s, ok := tag.(string); ok {
						opts.Tags = append(opts.Tags, s)
					}
				}
			}
			if md, ok := input["metadata"].(map[string]any); ok {
				opts.Metadata = md
			}

			task := defaultManager.CreateTask(opts)

			defaultMonitor.Notify(TaskEvent{
				TaskID:    task.ID,
				EventType: EventCreated,
				Timestamp: task.CreatedAt,
				Data:      map[string]any{"task": task},
			})

			return map[string]any{
				strconst.StrTaskId: task.ID,
				strconst.StrTitle:  task.Title,
				strconst.StrStatus: task.Status.String(),
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerTaskUpdate(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_update",
		Description: "Update a task's status, progress, result, or metadata.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrTaskId: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task ID to update",
				},
				strconst.StrStatus: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "New status: pending, running, completed, failed, cancelled, timeout",
					"enum":                  []string{strconst.StrPending, strconst.StrRunning, strconst.StrCompleted, strconst.StrFailed, strconst.StrCancelled, strconst.StrTimeout},
				},
				strconst.StrProgress: map[string]any{
					"type":                  "number",
					strconst.StrDescription: "Progress as a float between 0.0 and 1.0",
				},
				strconst.StrResult: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task result text",
				},
				strconst.StrError: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Error message if task failed",
				},
				"metadata": map[string]any{
					"type":                  strconst.StrObject,
					strconst.StrDescription: "Metadata to merge into the task",
				},
			},
			strconst.StrRequired: []string{strconst.StrTaskId},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrTaskId] == nil {
				return fmt.Errorf("task_id is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			taskID := input[strconst.StrTaskId].(string)
			update := TaskUpdate{}

			if s, ok := input[strconst.StrStatus].(string); ok {
				status, err := ParseTaskStatus(s)
				if err != nil {
					return nil, err
				}
				update.Status = &status
			}
			if p, ok := input[strconst.StrProgress].(float64); ok {
				update.Progress = &p
			}
			if r, ok := input[strconst.StrResult].(string); ok {
				update.Result = &r
			}
			if e, ok := input[strconst.StrError].(string); ok {
				update.Error = &e
			}
			if md, ok := input["metadata"].(map[string]any); ok {
				update.Metadata = md
			}

			if err := defaultManager.UpdateTask(taskID, update); err != nil {
				return nil, err
			}

			task, _ := defaultManager.GetTask(taskID)
			if update.Status != nil {
				var eventType TaskEventType
				switch *update.Status {
				case StatusRunning:
					eventType = EventStarted
				case StatusCompleted:
					eventType = EventCompleted
				case StatusFailed:
					eventType = EventFailed
				case StatusCancelled:
					eventType = EventCancelled
				case StatusTimeout:
					eventType = EventTimeout
				default:
					eventType = EventProgress
				}
				defaultMonitor.Notify(TaskEvent{
					TaskID:    taskID,
					EventType: eventType,
					Timestamp: task.UpdatedAt,
					Data:      map[string]any{"task": task},
				})
			}

			return map[string]any{
				strconst.StrTaskId: task.ID,
				strconst.StrStatus: task.Status.String(),
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerTaskOutput(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_output",
		Description: "Retrieve captured output (stdout, stderr, files) from a completed or running task.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrTaskId: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task ID to retrieve output for",
				},
			},
			strconst.StrRequired: []string{strconst.StrTaskId},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrTaskId] == nil {
				return fmt.Errorf("task_id is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			taskID := input[strconst.StrTaskId].(string)
			output, err := defaultOutputMgr.GetOutput(taskID)
			if err != nil {
				//nolint:nilerr // report the error inside the tool result.
				return map[string]any{
					strconst.StrTaskId: taskID,
					strconst.StrError:  err.Error(),
					strconst.StrStdout: "",
					strconst.StrStderr: "",
				}, nil
			}
			return map[string]any{
				strconst.StrTaskId: output.TaskID,
				strconst.StrStdout: output.Stdout,
				strconst.StrStderr: output.Stderr,
				"exit_code":        output.ExitCode,
			}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerTaskList(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_list",
		Description: "List tasks with optional filters for status, priority, assignment, and pagination.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrStatus: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Filter by status",
					"enum":                  []string{strconst.StrPending, strconst.StrRunning, strconst.StrCompleted, strconst.StrFailed, strconst.StrCancelled, strconst.StrTimeout},
				},
				strconst.StrPriority: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Filter by priority",
					"enum":                  []string{"low", strconst.StrNormal, "high", strconst.StrUrgent},
				},
				strconst.StrAssignedTo: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Filter by assigned agent/worker",
				},
				"parent_id": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Filter by parent task ID",
				},
				"limit": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "Max results to return",
					strconst.StrMinimum:     1,
				},
				"offset": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "Result offset for pagination",
					strconst.StrMinimum:     0,
				},
				"sort_by": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Sort field: created_at, updated_at, title, priority, status",
					"enum":                  []string{strconst.StrCreatedAt, "updated_at", strconst.StrTitle, strconst.StrPriority, strconst.StrStatus},
				},
				"sort_dir": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Sort direction: asc or desc",
					"enum":                  []string{"asc", "desc"},
				},
			},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			opts := ListTaskOpts{}
			opts.Status = toString(input[strconst.StrStatus])
			opts.Priority = toString(input[strconst.StrPriority])
			opts.AssignedTo = toString(input[strconst.StrAssignedTo])
			opts.ParentID = toString(input["parent_id"])
			opts.SortBy = toString(input["sort_by"])
			opts.SortDir = toString(input["sort_dir"])

			if v, ok := input["limit"].(float64); ok {
				opts.Limit = int(v)
			}
			if v, ok := input["offset"].(float64); ok {
				opts.Offset = int(v)
			}

			tasks := defaultManager.ListTasks(opts)
			result := make([]map[string]any, len(tasks))
			for i, task := range tasks {
				result[i] = map[string]any{
					"id":                   task.ID,
					strconst.StrTitle:      task.Title,
					strconst.StrStatus:     task.Status.String(),
					strconst.StrPriority:   task.Priority.String(),
					strconst.StrAssignedTo: task.AssignedTo,
					strconst.StrProgress:   task.Progress,
					strconst.StrCreatedAt:  task.CreatedAt,
				}
			}
			return map[string]any{
				"tasks":           result,
				strconst.StrCount: len(result),
			}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerTaskCancel(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_cancel",
		Description: "Cancel a pending or running task.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrTaskId: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task ID to cancel",
				},
			},
			strconst.StrRequired: []string{strconst.StrTaskId},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrTaskId] == nil {
				return fmt.Errorf("task_id is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			taskID := input[strconst.StrTaskId].(string)

			task, ok := defaultManager.GetTask(taskID)
			if !ok {
				return nil, fmt.Errorf("task %q not found", taskID)
			}

			if err := defaultManager.CancelTask(taskID); err != nil {
				return nil, err
			}

			defaultMonitor.Notify(TaskEvent{
				TaskID:    taskID,
				EventType: EventCancelled,
				Timestamp: task.UpdatedAt,
				Data:      map[string]any{"task": task},
			})

			return map[string]any{
				strconst.StrTaskId: taskID,
				strconst.StrStatus: strconst.StrCancelled,
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerTaskWait(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "task_wait",
		Description: "Block until a task reaches a terminal state (completed, failed, cancelled, timeout).",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrTaskId: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Task ID to wait for",
				},
				strconst.StrTimeout: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Max wait duration as Go duration string (e.g. 30s, 5m)",
				},
			},
			strconst.StrRequired: []string{strconst.StrTaskId},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrTaskId] == nil {
				return fmt.Errorf("task_id is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			taskID := input[strconst.StrTaskId].(string)
			var timeout = 30 * time.Second

			if t, ok := input[strconst.StrTimeout].(string); ok && t != "" {
				d, err := parseDuration(t)
				if err != nil {
					return nil, fmt.Errorf("invalid timeout: %w", err)
				}
				timeout = time.Duration(d)
			}

			task, err := defaultMonitor.WaitFor(taskID, timeout)
			if err != nil {
				return nil, err
			}

			return map[string]any{
				strconst.StrTaskId: task.ID,
				strconst.StrStatus: task.Status.String(),
				strconst.StrResult: task.Result,
				strconst.StrError:  task.Error,
			}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func parseDuration(s string) (int64, error) {
	var val int64
	var unit byte

	n, err := fmt.Sscanf(s, "%d%c", &val, &unit)
	if err != nil || n < 1 {
		n2, err2 := fmt.Sscanf(s, "%d", &val)
		if err2 != nil || n2 < 1 {
			return 0, fmt.Errorf("cannot parse duration %q", s)
		}
		return val * int64(1_000_000_000), nil
	}

	switch unit {
	case 's':
		return val * int64(1_000_000_000), nil
	case 'm':
		return val * int64(60_000_000_000), nil
	case 'h':
		return val * int64(3_600_000_000_000), nil
	case 'd':
		return val * int64(86_400_000_000_000), nil
	default:
		return 0, fmt.Errorf("unknown duration unit %q", string(unit))
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
