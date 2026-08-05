// SPDX-License-Identifier: MIT
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// CommandResult holds the output of a remote command execution.
type CommandResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration int64  `json:"duration_ms"`
}

// ShellRequest is the payload for a shell command execution.
type ShellRequest struct {
	Command string `json:"command"`
	WorkDir string `json:"work_dir,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// FileReadRequest is the payload for a remote file read.
type FileReadRequest struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}

// FileWriteRequest is the payload for a remote file write.
type FileWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    uint32 `json:"mode,omitempty"`
}

// FileListRequest is the payload for a remote directory listing.
type FileListRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
}

// FileEntry represents a file or directory entry.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	Mode    uint32 `json:"mode"`
}

// ClipboardRequest is the payload for a clipboard operation.
type ClipboardRequest struct {
	Action  string `json:"action"`
	Content string `json:"content,omitempty"`
}

// ClipboardResult holds clipboard operation output.
type ClipboardResult struct {
	Content string `json:"content"`
}

// RemoteControl provides methods for executing commands on a remote instance.
type RemoteControl struct {
	session *RemoteSession
}

// NewRemoteControl creates a controller bound to a session.
func NewRemoteControl(session *RemoteSession) *RemoteControl {
	return &RemoteControl{session: session}
}

// Shell executes a shell command on the remote instance.
func (rc *RemoteControl) Shell(ctx context.Context, command, workDir string, timeout int) (*CommandResult, error) {
	req := ShellRequest{
		Command: command,
		WorkDir: workDir,
		Timeout: timeout,
	}

	resp, err := rc.session.SendRequest(ctx, "control.shell", req)
	if err != nil {
		return nil, fmt.Errorf("shell request: %w", err)
	}

	var result CommandResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return nil, fmt.Errorf("decode shell result: %w", err)
	}
	return &result, nil
}

// ShellWithTimeout executes a shell command with a timeout.
func (rc *RemoteControl) ShellWithTimeout(ctx context.Context, command, workDir string, timeoutSec int) (*CommandResult, error) {
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}
	return rc.Shell(ctx, command, workDir, timeoutSec)
}

// FileRead reads a file from the remote instance.
func (rc *RemoteControl) FileRead(ctx context.Context, path string, offset, limit int64) (*CommandResult, error) {
	req := FileReadRequest{
		Path:   path,
		Offset: offset,
		Limit:  limit,
	}

	resp, err := rc.session.SendRequest(ctx, "control.file.read", req)
	if err != nil {
		return nil, fmt.Errorf("file read request: %w", err)
	}

	var result CommandResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return nil, fmt.Errorf("decode file read result: %w", err)
	}
	return &result, nil
}

// FileWrite writes content to a file on the remote instance.
func (rc *RemoteControl) FileWrite(ctx context.Context, path, content string, mode uint32) (*CommandResult, error) {
	req := FileWriteRequest{
		Path:    path,
		Content: content,
		Mode:    mode,
	}

	resp, err := rc.session.SendRequest(ctx, "control.file.write", req)
	if err != nil {
		return nil, fmt.Errorf("file write request: %w", err)
	}

	var result CommandResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return nil, fmt.Errorf("decode file write result: %w", err)
	}
	return &result, nil
}

// FileList lists files in a directory on the remote instance.
func (rc *RemoteControl) FileList(ctx context.Context, path string, recursive bool, pattern string) ([]FileEntry, error) {
	req := FileListRequest{
		Path:      path,
		Recursive: recursive,
		Pattern:   pattern,
	}

	resp, err := rc.session.SendRequest(ctx, "control.file.list", req)
	if err != nil {
		return nil, fmt.Errorf("file list request: %w", err)
	}

	var entries []FileEntry
	if err := resp.UnmarshalPayload(&entries); err != nil {
		return nil, fmt.Errorf("decode file list result: %w", err)
	}
	return entries, nil
}

// ClipboardGet retrieves the clipboard content from the remote instance.
func (rc *RemoteControl) ClipboardGet(ctx context.Context) (string, error) {
	req := ClipboardRequest{Action: "get"}

	resp, err := rc.session.SendRequest(ctx, "control.clipboard", req)
	if err != nil {
		return "", fmt.Errorf("clipboard get request: %w", err)
	}

	var result ClipboardResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return "", fmt.Errorf("decode clipboard result: %w", err)
	}
	return result.Content, nil
}

// ClipboardSet sets the clipboard content on the remote instance.
func (rc *RemoteControl) ClipboardSet(ctx context.Context, content string) error {
	req := ClipboardRequest{
		Action:  "set",
		Content: content,
	}

	_, err := rc.session.SendRequest(ctx, "control.clipboard", req)
	if err != nil {
		return fmt.Errorf("clipboard set request: %w", err)
	}
	return nil
}

// Command executes an arbitrary command with a JSON payload.
func (rc *RemoteControl) Command(ctx context.Context, method string, payload any) (*CommandResult, error) {
	resp, err := rc.session.SendRequest(ctx, method, payload)
	if err != nil {
		return nil, fmt.Errorf("command request: %w", err)
	}

	var result CommandResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return nil, fmt.Errorf("decode command result: %w", err)
	}
	return &result, nil
}

// CommandRaw executes a command and returns the raw message response.
func (rc *RemoteControl) CommandRaw(ctx context.Context, method string, payload any) (*RemoteMessage, error) {
	return rc.session.SendRequest(ctx, method, payload)
}

// RegisterControlHandlers registers default control handlers on a router.
func RegisterControlHandlers(router *MessageRouter, handler *RemoteControl) {
	router.RegisterFunc("control.shell", func(msg *RemoteMessage) (*RemoteMessage, error) {
		var req ShellRequest
		if err := msg.UnmarshalPayload(&req); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if req.Timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)
			defer cancel()
		}
		result, err := handler.Shell(ctx, req.Command, req.WorkDir, req.Timeout)
		if err != nil {
			return NewErrorMessage(msg.ID, 500, "shell error", err.Error()), nil
		}
		return NewResponse(msg.ID, result)
	})

	router.RegisterFunc("control.file.read", func(msg *RemoteMessage) (*RemoteMessage, error) {
		var req FileReadRequest
		if err := msg.UnmarshalPayload(&req); err != nil {
			return nil, err
		}
		result, err := handler.FileRead(context.Background(), req.Path, req.Offset, req.Limit)
		if err != nil {
			return NewErrorMessage(msg.ID, 500, "file read error", err.Error()), nil
		}
		return NewResponse(msg.ID, result)
	})

	router.RegisterFunc("control.file.write", func(msg *RemoteMessage) (*RemoteMessage, error) {
		var req FileWriteRequest
		if err := msg.UnmarshalPayload(&req); err != nil {
			return nil, err
		}
		result, err := handler.FileWrite(context.Background(), req.Path, req.Content, req.Mode)
		if err != nil {
			return NewErrorMessage(msg.ID, 500, "file write error", err.Error()), nil
		}
		return NewResponse(msg.ID, result)
	})

	router.RegisterFunc("control.file.list", func(msg *RemoteMessage) (*RemoteMessage, error) {
		var req FileListRequest
		if err := msg.UnmarshalPayload(&req); err != nil {
			return nil, err
		}
		entries, err := handler.FileList(context.Background(), req.Path, req.Recursive, req.Pattern)
		if err != nil {
			return NewErrorMessage(msg.ID, 500, "file list error", err.Error()), nil
		}
		return NewResponse(msg.ID, entries)
	})

	router.RegisterFunc("control.clipboard", func(msg *RemoteMessage) (*RemoteMessage, error) {
		var req ClipboardRequest
		if err := msg.UnmarshalPayload(&req); err != nil {
			return nil, err
		}
		switch req.Action {
		case "get":
			content, err := handler.ClipboardGet(context.Background())
			if err != nil {
				return NewErrorMessage(msg.ID, 500, "clipboard get error", err.Error()), nil
			}
			return NewResponse(msg.ID, ClipboardResult{Content: content})
		case "set":
			err := handler.ClipboardSet(context.Background(), req.Content)
			if err != nil {
				return NewErrorMessage(msg.ID, 500, "clipboard set error", err.Error()), nil
			}
			return NewResponse(msg.ID, map[string]string{strconst.StrStatus: "ok"})
		default:
			return NewErrorMessage(msg.ID, 400, "unknown clipboard action", req.Action), nil
		}
	})
}

// ---- Batch Operations ----

// BatchRequest contains multiple control operations to execute.
type BatchRequest struct {
	Operations []BatchOperation `json:"operations"`
}

// BatchOperation is a single operation within a batch.
type BatchOperation struct {
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

// BatchResult holds the result of a batch operation.
type BatchResult struct {
	Results []BatchOperationResult `json:"results"`
}

// BatchOperationResult holds the result of a single operation in a batch.
type BatchOperationResult struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Batch executes multiple operations in sequence on the remote instance.
func (rc *RemoteControl) Batch(ctx context.Context, operations []BatchOperation) (*BatchResult, error) {
	req := BatchRequest{Operations: operations}
	resp, err := rc.session.SendRequest(ctx, "control.batch", req)
	if err != nil {
		return nil, fmt.Errorf("batch request: %w", err)
	}

	var result BatchResult
	if err := resp.UnmarshalPayload(&result); err != nil {
		return nil, fmt.Errorf("decode batch result: %w", err)
	}
	return &result, nil
}
