package tasktools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// TaskOutput holds the captured output from a task execution.
type TaskOutput struct {
	TaskID         string         `json:"task_id"`
	Stdout         string         `json:"stdout"`
	Stderr         string         `json:"stderr"`
	ExitCode       int            `json:"exit_code"`
	Files          []OutputFile   `json:"files,omitempty"`
	StructuredData map[string]any `json:"structured_data,omitempty"`
}

// OutputFile represents a file produced by a task.
type OutputFile struct {
	Path     string `json:"path"`
	Content  string `json:"content,omitempty"`
	Size     int64  `json:"size"`
	MIMEType string `json:"mime_type,omitempty"`
}

// OutputMatch represents a line match from output search.
type OutputMatch struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Stream  string `json:"stream"`
}

// OutputStore defines how task outputs are persisted.
type OutputStore interface {
	Save(taskID string, output *TaskOutput) error
	Load(taskID string) (*TaskOutput, error)
	Append(taskID string, data string, stream string) error
}

// MemoryOutputStore keeps task outputs in memory with per-task size limits.
type MemoryOutputStore struct {
	mu       sync.RWMutex
	outputs  map[string]*TaskOutput
	maxSize  int
	lineBuff map[string][]string
}

// NewMemoryOutputStore creates an in-memory output store.
// maxSize is the max lines kept per stream per task.
func NewMemoryOutputStore(maxSize int) *MemoryOutputStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &MemoryOutputStore{
		outputs:  make(map[string]*TaskOutput),
		maxSize:  maxSize,
		lineBuff: make(map[string][]string),
	}
}

func (s *MemoryOutputStore) Save(taskID string, output *TaskOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputs[taskID] = output
	return nil
}

func (s *MemoryOutputStore) Load(taskID string) (*TaskOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out, ok := s.outputs[taskID]
	if !ok {
		return nil, fmt.Errorf("output for task %q not found", taskID)
	}
	return out, nil
}

func (s *MemoryOutputStore) Append(taskID string, data string, stream string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	out, ok := s.outputs[taskID]
	if !ok {
		out = &TaskOutput{TaskID: taskID, StructuredData: make(map[string]any)}
		s.outputs[taskID] = out
	}

	key := taskID + ":" + stream
	lines := strings.Split(data, "\n")
	s.lineBuff[key] = append(s.lineBuff[key], lines...)

	if len(s.lineBuff[key]) > s.maxSize {
		s.lineBuff[key] = s.lineBuff[key][len(s.lineBuff[key])-s.maxSize:]
	}

	joined := strings.Join(s.lineBuff[key], "\n")
	switch stream {
	case strconst.StrStdout:
		out.Stdout = joined
	case strconst.StrStderr:
		out.Stderr = joined
	default:
		out.Stdout = joined
	}
	return nil
}

// FileOutputStore persists task outputs to disk.
type FileOutputStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileOutputStore creates a file-based output store under the given directory.
func NewFileOutputStore(baseDir string) *FileOutputStore {
	return &FileOutputStore{baseDir: baseDir}
}

func (s *FileOutputStore) taskDir(taskID string) string {
	return filepath.Join(s.baseDir, taskID)
}

func (s *FileOutputStore) Save(taskID string, output *TaskOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.taskDir(taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}

	for _, f := range output.Files {
		if f.Content != "" {
			if err := os.WriteFile(filepath.Join(dir, filepath.Base(f.Path)), []byte(f.Content), 0o644); err != nil {
				return fmt.Errorf("write file %q: %w", f.Path, err)
			}
		}
	}

	return nil
}

func (s *FileOutputStore) Load(taskID string) (*TaskOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := s.taskDir(taskID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("output for task %q not found", taskID)
	}

	out := &TaskOutput{TaskID: taskID, StructuredData: make(map[string]any)}

	stdoutPath := filepath.Join(dir, "stdout.log")
	if data, err := os.ReadFile(stdoutPath); err == nil {
		out.Stdout = string(data)
	}

	stderrPath := filepath.Join(dir, "stderr.log")
	if data, err := os.ReadFile(stderrPath); err == nil {
		out.Stderr = string(data)
	}

	return out, nil
}

func (s *FileOutputStore) Append(taskID string, data string, stream string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.taskDir(taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}

	fileName := "stdout.log"
	if stream == strconst.StrStderr {
		fileName = "stderr.log"
	}

	path := filepath.Join(dir, fileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(data)
	return err
}

// TaskOutputManager coordinates output capture and retrieval.
type TaskOutputManager struct {
	store OutputStore
	tasks *TaskManager
}

// NewTaskOutputManager creates a new output manager.
func NewTaskOutputManager(store OutputStore) *TaskOutputManager {
	return &TaskOutputManager{
		store: store,
		tasks: NewTaskManager(),
	}
}

// SetTaskManager sets the task manager reference for the output manager.
func (m *TaskOutputManager) SetTaskManager(tm *TaskManager) {
	m.tasks = tm
}

// CaptureOutput runs a command and captures its stdout and stderr.
func (m *TaskOutputManager) CaptureOutput(taskID string, cmd *exec.Cmd) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	var stdout, stderr strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdout.WriteString(line)
			stdout.WriteString("\n")
			_ = m.store.Append(taskID, line+"\n", strconst.StrStdout)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderr.WriteString(line)
			stderr.WriteString("\n")
			_ = m.store.Append(taskID, line+"\n", strconst.StrStderr)
		}
	}()

	wg.Wait()

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("wait: %w", err)
		}
	}

	output := &TaskOutput{
		TaskID:   taskID,
		Stdout:   strings.TrimRight(stdout.String(), "\n"),
		Stderr:   strings.TrimRight(stderr.String(), "\n"),
		ExitCode: exitCode,
	}

	return m.store.Save(taskID, output)
}

// StreamOutput reads from a reader and appends to the task's output in real-time.
func (m *TaskOutputManager) StreamOutput(taskID string, reader io.Reader, stream string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		_ = m.store.Append(taskID, line+"\n", stream)
	}
}

// GetOutput retrieves the stored output for a task.
func (m *TaskOutputManager) GetOutput(taskID string) (*TaskOutput, error) {
	return m.store.Load(taskID)
}

// TruncateOutput truncates the stored output to maxLines.
func (m *TaskOutputManager) TruncateOutput(taskID string, maxLines int) error {
	out, err := m.store.Load(taskID)
	if err != nil {
		return err
	}

	truncate := func(s string) string {
		lines := strings.Split(s, "\n")
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		return strings.Join(lines, "\n")
	}

	out.Stdout = truncate(out.Stdout)
	out.Stderr = truncate(out.Stderr)
	return m.store.Save(taskID, out)
}

// SearchOutput searches task output for a query string.
func (m *TaskOutputManager) SearchOutput(taskID string, query string) []OutputMatch {
	out, err := m.store.Load(taskID)
	if err != nil {
		return nil
	}

	var matches []OutputMatch
	query = strings.ToLower(query)

	searchStream := func(content, stream string) {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), query) {
				matches = append(matches, OutputMatch{
					Line:    i + 1,
					Content: line,
					Stream:  stream,
				})
			}
		}
	}

	searchStream(out.Stdout, strconst.StrStdout)
	searchStream(out.Stderr, strconst.StrStderr)
	return matches
}
