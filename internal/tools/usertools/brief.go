package usertools

import (
	"fmt"
	"os"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const BriefName = "send_user_message"

type BriefInput struct {
	Message     string   `json:"message"`
	Attachments []string `json:"attachments,omitempty"`
}

type BriefResult struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	SentAt  string           `json:"sent_at,omitempty"`
	Files   []AttachmentInfo `json:"files,omitempty"`
	Error   string           `json:"error,omitempty"`
}

type AttachmentInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsImage bool   `json:"is_image"`
}

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".svg": true,
	".bmp": true, ".ico": true,
}

func BuildBriefTool(handler UserHandler) (tools.Tool, error) {
	return tools.Build(tools.ToolDef{
		Name:        BriefName,
		Description: "Send a message to the user, optionally with file attachments. Use this for status updates, summaries, or any proactive communication.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"message": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The message content to display to the user. Supports markdown.",
				},
				"attachments": map[string]any{
					"type":                  strconst.StrArray,
					strconst.StrDescription: "Optional file paths to attach (absolute or relative to cwd). Use for screenshots, diffs, logs, or any file the user should see.",
					strconst.StrItems: map[string]any{
						"type": strconst.StrString,
					},
				},
			},
			strconst.StrRequired: []string{"message"},
		},
		Validate: func(input map[string]any) error {
			msg, ok := input["message"]
			if !ok || msg == "" {
				return fmt.Errorf("message is required")
			}
			if attachments, ok := input["attachments"].([]any); ok {
				for i, a := range attachments {
					path, ok := a.(string)
					if !ok || path == "" {
						return fmt.Errorf("attachment %d: invalid path", i)
					}
				}
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			message := fmt.Sprint(input["message"])
			var attachments []string
			if atts, ok := input["attachments"].([]any); ok {
				for _, a := range atts {
					if s, ok := a.(string); ok && s != "" {
						attachments = append(attachments, s)
					}
				}
			}

			var files []AttachmentInfo
			for _, path := range attachments {
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				ext := ""
				if len(path) > 3 {
					for i := len(path) - 1; i >= 0; i-- {
						if path[i] == '.' {
							ext = path[i:]
							break
						}
					}
				}
				files = append(files, AttachmentInfo{
					Path:    path,
					Size:    info.Size(),
					IsImage: imageExtensions[ext],
				})
			}

			if err := handler.SendMessage(ctx.Context, message, files); err != nil {
				//nolint:nilerr // report the error inside the tool result.
				return BriefResult{
					Success: false,
					Error:   err.Error(),
				}, nil
			}

			result := BriefResult{
				Success: true,
				Message: message,
				SentAt:  time.Now().UTC().Format(time.RFC3339),
			}
			if len(files) > 0 {
				result.Files = files
			}
			return result, nil
		},
		IsReadOnly:       boolPtr(true),
		IsConcurrentSafe: boolPtr(true),
	})
}
