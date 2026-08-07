package synthtools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// REPOpts configures a REPLEngine.
type REPOpts struct {
	Timeout   time.Duration
	MaxOutput int
	Sandbox   bool
	EnvVars   map[string]string
}

// EvalResult holds the outcome of a code evaluation.
type EvalResult struct {
	Output   string        `json:"output"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	ExitCode int           `json:"exit_code"`
}

// REPLEngine provides sandboxed code evaluation.
type REPLEngine struct {
	language  string
	timeout   time.Duration
	maxOutput int
	sandbox   bool
	envVars   map[string]string
}

func NewREPLEngine(language string, opts REPOpts) *REPLEngine {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxOutput <= 0 {
		opts.MaxOutput = 1 << 20
	}
	return &REPLEngine{language: strings.ToLower(language), timeout: opts.Timeout, maxOutput: opts.MaxOutput, sandbox: opts.Sandbox, envVars: opts.EnvVars}
}

func SupportedLanguages() []string {
	return []string{"go", strconst.StrPython, strconst.StrJavascript, "bash", "bc"}
}

func (r *REPLEngine) buildCommand(code string) (string, []string, error) {
	switch r.language {
	case "go":
		return "sh", []string{"-c", fmt.Sprintf("tmpfile=$(mktemp /tmp/repl_XXXXXX.go); echo %q > \"$tmpfile\"; go run \"$tmpfile\"; rc=$?; rm -f \"$tmpfile\"; exit $rc", code)}, nil
	case strconst.StrPython:
		return "python3", []string{"-c", code}, nil
	case strconst.StrJavascript:
		return "node", []string{"-e", code}, nil
	case "bash":
		return "bash", []string{"-c", code}, nil
	case "bc":
		return "bc", []string{"-l"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported language %q", r.language)
	}
}

func (r *REPLEngine) Evaluate(expression string) (*EvalResult, error) {
	return r.Execute(expression)
}

func (r *REPLEngine) Execute(code string) (*EvalResult, error) {
	if code == "" {
		return nil, fmt.Errorf("code cannot be empty")
	}
	if err := ValidateCode(code, r.language); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}
	bin, args, err := r.buildCommand(code)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if len(r.envVars) > 0 {
		cmd.Env = make([]string, 0, len(r.envVars))
		for k, v := range r.envVars {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("exec: %w", err)
		}
	}
	stdoutStr, stderrStr := stdout.String(), stderr.String()
	if len(stdoutStr) > r.maxOutput {
		stdoutStr = stdoutStr[:r.maxOutput] + "\n... (truncated)"
	}
	if len(stderrStr) > r.maxOutput {
		stderrStr = stderrStr[:r.maxOutput] + "\n... (truncated)"
	}
	return &EvalResult{Output: stdoutStr, Error: stderrStr, Duration: duration, ExitCode: exitCode}, nil
}

func ValidateCode(code string, language string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("code is empty")
	}
	switch strings.ToLower(language) {
	case "go", strconst.StrPython, strconst.StrJavascript, "bash", "bc":
		return nil
	default:
		return fmt.Errorf("unsupported language %q", language)
	}
}

func GetHelp(language string) string {
	switch strings.ToLower(language) {
	case "go":
		return "Go snippets are written to a temp file and executed via `go run`. Must include a `main()` function."
	case strconst.StrPython:
		return "Python snippets are executed via `python3 -c`. Supports any valid Python expression or statement."
	case strconst.StrJavascript:
		return "JavaScript snippets are executed via `node -e`. Supports any valid JS expression or statement."
	case "bash":
		return "Bash snippets are executed via `bash -c`. Supports any valid shell command."
	case "bc":
		return "Bc expressions are piped to `bc -l`. Supports arithmetic and math functions."
	default:
		return fmt.Sprintf("No help available for %q.", language)
	}
}
