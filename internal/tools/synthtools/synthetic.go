package synthtools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// OutputType describes the format of a synthetic output.
type OutputType int

const (
	OutputJSON OutputType = iota
	OutputText
	OutputMarkdown
	OutputError
	OutputProgress
)

func (ot OutputType) String() string {
	return [...]string{"json", "text", strconst.StrMarkdown, strconst.StrError, strconst.StrProgress}[ot]
}

func ParseOutputType(s string) (OutputType, error) {
	switch strings.ToLower(s) {
	case "json":
		return OutputJSON, nil
	case "text":
		return OutputText, nil
	case strconst.StrMarkdown:
		return OutputMarkdown, nil
	case strconst.StrError:
		return OutputError, nil
	case strconst.StrProgress:
		return OutputProgress, nil
	default:
		return OutputText, fmt.Errorf("unknown output type %q", s)
	}
}

type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type MarkdownSection struct {
	Heading   string     `json:"heading"`
	Content   string     `json:"content,omitempty"`
	Level     int        `json:"level"`
	CodeBlock *CodeBlock `json:"code_block,omitempty"`
}

type SyntheticOutput struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Data      string         `json:"data"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func GenerateJSON(data map[string]any, pretty bool) string {
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(data, "", "  ")
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

func GenerateText(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

func GenerateMarkdown(title string, sections []MarkdownSection) string {
	var sb strings.Builder
	if title != "" {
		sb.WriteString("# ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	for _, sec := range sections {
		sb.WriteString(strings.Repeat("#", sec.Level))
		sb.WriteString(" ")
		sb.WriteString(sec.Heading)
		sb.WriteString("\n\n")
		if sec.Content != "" {
			sb.WriteString(sec.Content)
			sb.WriteString("\n\n")
		}
		if sec.CodeBlock != nil {
			sb.WriteString("```")
			sb.WriteString(sec.CodeBlock.Language)
			sb.WriteString("\n")
			sb.WriteString(sec.CodeBlock.Code)
			sb.WriteString("\n```\n\n")
		}
	}
	return sb.String()
}

func GenerateError(code string, message string, details map[string]any) string {
	payload := map[string]any{
		strconst.StrError: map[string]any{"code": code, "message": message},
	}
	if len(details) > 0 {
		payload[strconst.StrError].(map[string]any)["details"] = details
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"error":{"code":"%s","message":%q}}`, code, err.Error())
	}
	return string(b)
}

func GenerateProgress(current, total int, message string) string {
	if total <= 0 {
		return fmt.Sprintf("[%s] progress: %d", message, current)
	}
	pct := float64(current) / float64(total) * 100
	barWidth, filled := 20, int(pct/100*20)
	if filled > barWidth {
		filled = barWidth
	}
	return fmt.Sprintf("[%s%s] %s %.0f%% (%d/%d)",
		strings.Repeat("█", filled), strings.Repeat("░", barWidth-filled),
		message, pct, current, total)
}

func ValidateOutput(output string, format OutputType) error {
	if format == OutputJSON {
		var v any
		if err := json.Unmarshal([]byte(output), &v); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	}
	return nil
}

func ParseJSONOutput(output string) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return result, nil
}
