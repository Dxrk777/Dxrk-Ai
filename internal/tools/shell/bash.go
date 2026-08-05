package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func RegisterAll(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "shell",
		Description: "Execute bash commands with timeout and validation",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"command": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Bash command to execute",
				},
				strconst.StrTimeout: map[string]any{
					"type":                  "number",
					strconst.StrDescription: "Timeout in seconds (default 30)",
					"default":               30,
				},
				"workdir": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Working directory for the command",
				},
			},
			strconst.StrRequired: []string{"command"},
		},
		Execute: execute,
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func execute(ctx tools.Context, args map[string]any) (any, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return map[string]any{
			strconst.StrSuccess: false,
			strconst.StrError:   "command is required",
		}, nil
	}

	timeout := 30
	if t, ok := args[strconst.StrTimeout].(float64); ok && t > 0 {
		timeout = int(t)
	}

	workdir, _ := args["workdir"].(string)
	if workdir == "" {
		if pwd, ok := os.LookupEnv("PWD"); ok {
			workdir = pwd
		} else {
			workdir, _ = os.Getwd()
		}
	}

	validation := ValidateCommand(command)
	if !validation.IsValid {
		return map[string]any{
			strconst.StrSuccess: false,
			strconst.StrError:   validation.Reason,
			"risk_level":        validation.RiskLevel,
		}, nil
	}

	sandbox := NewSandbox(DefaultSandboxConfig())
	cmdCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
	cmd.Dir = workdir
	cmd.Env = os.Environ()

	cmd = sandbox.WrapCommand(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	result := map[string]any{
		strconst.StrSuccess: exitCode == 0,
		strconst.StrStdout:  stdout.String(),
		strconst.StrStderr:  stderr.String(),
		"exit_code":         exitCode,
		"workdir":           workdir,
		"risk_level":        validation.RiskLevel,
	}

	if len(validation.Warnings) > 0 {
		result["warnings"] = validation.Warnings
	}

	if cmdCtx.Err() == context.DeadlineExceeded {
		result[strconst.StrSuccess] = false
		result[strconst.StrError] = fmt.Sprintf("command timed out after %d seconds", timeout)
	}

	return result, nil
}

func ShellQuote(args []string) string {
	var quoted []string
	for _, arg := range args {
		if needsQuoting(arg) {
			quoted = append(quoted, fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
		} else {
			quoted = append(quoted, arg)
		}
	}
	return strings.Join(quoted, " ")
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\'' || r == '"' || r == '\\' || r == '$' || r == '`' || r == '|' || r == '&' || r == ';' || r == '(' || r == ')' || r == '{' || r == '}' || r == '<' || r == '>' || r == '!' || r == '#' || r == '~' {
			return true
		}
	}
	return false
}
