package synthtools

import (
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

// RegisterAll registers all synthtools into the given registry.
func RegisterAll(reg *tools.Registry) error {
	for _, fn := range []func(*tools.Registry) error{
		registerSyntheticOutput,
		registerSleep,
		registerREPL,
		registerDataTransform,
	} {
		if err := fn(reg); err != nil {
			return err
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }

func getInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

func getBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func registerSyntheticOutput(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "synthetic_output",
		Description: "Generate structured output for testing and validation without running commands",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFormat: map[string]any{
					"type": strconst.StrString, strconst.StrDescription: "Output format: json, text, markdown, error, progress",
					"enum": []string{"json", "text", strconst.StrMarkdown, strconst.StrError, strconst.StrProgress},
				},
				"data":             map[string]any{"type": strconst.StrObject, strconst.StrDescription: "Data for JSON output"},
				"pretty":           map[string]any{"type": "boolean", strconst.StrDescription: "Pretty-print JSON (default true)"},
				"template":         map[string]any{"type": strconst.StrString, strconst.StrDescription: "Text template with {{var}} placeholders"},
				"vars":             map[string]any{"type": strconst.StrObject, strconst.StrDescription: "Template variables"},
				strconst.StrTitle:  map[string]any{"type": strconst.StrString, strconst.StrDescription: "Markdown title"},
				"sections":         map[string]any{"type": strconst.StrArray, strconst.StrItems: map[string]any{"type": strconst.StrObject}},
				"error_code":       map[string]any{"type": strconst.StrString},
				"error_message":    map[string]any{"type": strconst.StrString},
				"error_details":    map[string]any{"type": strconst.StrObject},
				"current":          map[string]any{"type": strconst.StrInteger},
				"total":            map[string]any{"type": strconst.StrInteger},
				"progress_message": map[string]any{"type": strconst.StrString},
			},
			strconst.StrRequired: []string{strconst.StrFormat},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrFormat] == nil {
				return fmt.Errorf("format is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			format, err := ParseOutputType(fmt.Sprintf("%v", input[strconst.StrFormat]))
			if err != nil {
				return nil, err
			}
			var output string
			switch format {
			case OutputJSON:
				pretty := true
				if p, ok := input["pretty"].(bool); ok {
					pretty = p
				}
				output = GenerateJSON(input["data"].(map[string]any), pretty)
			case OutputText:
				vars := make(map[string]string)
				if raw, ok := input["vars"].(map[string]any); ok {
					for k, v := range raw {
						vars[k] = fmt.Sprintf("%v", v)
					}
				}
				output = GenerateText(fmt.Sprintf("%v", input["template"]), vars)
			case OutputMarkdown:
				output = GenerateMarkdown(fmt.Sprintf("%v", input[strconst.StrTitle]), parseMarkdownSections(input["sections"]))
			case OutputError:
				output = GenerateError(fmt.Sprintf("%v", input["error_code"]), fmt.Sprintf("%v", input["error_message"]), input["error_details"].(map[string]any))
			case OutputProgress:
				output = GenerateProgress(int(getInt64(input["current"])), int(getInt64(input["total"])), fmt.Sprintf("%v", input["progress_message"]))
			}
			return map[string]any{"output": output, "type": format.String()}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerSleep(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "sleep",
		Description: "Pause execution for a specified duration with interrupt support",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"duration_ms": map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Sleep duration in milliseconds"},
				"until":       map[string]any{"type": strconst.StrString, strconst.StrDescription: "Sleep until this RFC3339 timestamp"},
			},
		},
		Validate: func(input map[string]any) error {
			if input == nil || (input["duration_ms"] == nil && input["until"] == nil) {
				return fmt.Errorf("either duration_ms or until is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			st := NewSleepTool(5*time.Minute, true)
			if untilStr, ok := input["until"].(string); ok {
				until, err := time.Parse(time.RFC3339, untilStr)
				if err != nil {
					return nil, fmt.Errorf("parse until time: %w", err)
				}
				start := time.Now()
				if err := st.SleepUntil(ctx.Context, until); err != nil {
					return nil, fmt.Errorf("sleep interrupted: %w", err)
				}
				return map[string]any{"slept_ms": time.Since(start).Milliseconds(), "until": untilStr, strconst.StrCompleted: true}, nil
			}
			dur := time.Duration(getInt64(input["duration_ms"])) * time.Millisecond
			start := time.Now()
			if err := st.Sleep(ctx.Context, dur); err != nil {
				return nil, fmt.Errorf("sleep interrupted: %w", err)
			}
			return map[string]any{"slept_ms": time.Since(start).Milliseconds(), "duration": dur.String(), strconst.StrCompleted: true}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerREPL(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "repl",
		Description: "Evaluate code in a sandboxed environment (go, python, javascript, bash, bc)",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"language":   map[string]any{"type": strconst.StrString, strconst.StrDescription: "Programming language", "enum": []string{"go", strconst.StrPython, strconst.StrJavascript, "bash", "bc"}},
				"code":       map[string]any{"type": strconst.StrString, strconst.StrDescription: "Code to evaluate"},
				"timeout_ms": map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Execution timeout in ms (default 30000)"},
			},
			strconst.StrRequired: []string{"language", "code"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input["language"] == nil || input["code"] == nil {
				return fmt.Errorf("language and code are required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			lang, _ := input["language"].(string)
			code, _ := input["code"].(string)
			timeout := 30 * time.Second
			if ms, ok := input["timeout_ms"]; ok {
				timeout = time.Duration(getInt64(ms)) * time.Millisecond
			}
			result, err := NewREPLEngine(lang, REPOpts{Timeout: timeout}).Execute(code)
			if err != nil {
				return nil, err
			}
			return map[string]any{"output": result.Output, strconst.StrError: result.Error, "duration": result.Duration.String(), "exit_code": result.ExitCode}, nil
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerDataTransform(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "data_transform",
		Description: "Apply common data transformations (JSON query, CSV, base64, URL encode, hash, format)",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"operation": map[string]any{
					"type": strconst.StrString,
					"enum": []string{
						"json_query", "csv_transform", "yaml_query",
						"base64_encode", "base64_decode", "url_encode", "url_decode",
						"hash", "hmac", "format_duration", "format_bytes", "format_number",
						"uuid", "timestamp", "timestamp_unix",
					},
				},
				"input":            map[string]any{"type": strconst.StrString},
				strconst.StrQuery:  map[string]any{"type": strconst.StrString},
				"algorithm":        map[string]any{"type": strconst.StrString},
				"key":              map[string]any{"type": strconst.StrString},
				strconst.StrFormat: map[string]any{"type": strconst.StrString},
				"csv_delimiter":    map[string]any{"type": strconst.StrString},
				"csv_header":       map[string]any{"type": "boolean"},
				"csv_columns":      map[string]any{"type": strconst.StrArray, strconst.StrItems: map[string]any{"type": strconst.StrInteger}},
				"csv_sort_by":      map[string]any{"type": strconst.StrString},
				"csv_limit":        map[string]any{"type": strconst.StrInteger},
				"bytes":            map[string]any{"type": strconst.StrInteger},
				"number":           map[string]any{"type": strconst.StrInteger},
				"duration_ms":      map[string]any{"type": strconst.StrInteger},
			},
			strconst.StrRequired: []string{"operation"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input["operation"] == nil {
				return fmt.Errorf("operation is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			op, _ := input["operation"].(string)
			inp, _ := input["input"].(string)
			switch op {
			case "json_query":
				r, err := TransformJSON(inp, fmt.Sprintf("%v", input[strconst.StrQuery]))
				if err != nil {
					return nil, err
				}
				return map[string]any{strconst.StrResult: r}, nil
			case "csv_transform":
				opts := CSVOpts{Delimiter: ',', HasHeader: getBool(input["csv_header"])}
				if d, ok := input["csv_delimiter"].(string); ok && len(d) > 0 {
					opts.Delimiter = rune(d[0])
				}
				if cols, ok := input["csv_columns"].([]any); ok {
					for _, c := range cols {
						opts.Columns = append(opts.Columns, int(getInt64(c)))
					}
				}
				opts.SortBy, _ = input["csv_sort_by"].(string)
				opts.Limit = int(getInt64(input["csv_limit"]))
				r, err := TransformCSV(inp, opts)
				if err != nil {
					return nil, err
				}
				return map[string]any{strconst.StrResult: r}, nil
			case "yaml_query":
				r, err := TransformYAML(inp, fmt.Sprintf("%v", input[strconst.StrQuery]))
				if err != nil {
					return nil, err
				}
				return map[string]any{strconst.StrResult: r}, nil
			case "base64_encode":
				return map[string]any{strconst.StrResult: Base64Encode(inp)}, nil
			case "base64_decode":
				r, err := Base64Decode(inp)
				if err != nil {
					return nil, err
				}
				return map[string]any{strconst.StrResult: r}, nil
			case "url_encode":
				return map[string]any{strconst.StrResult: URLEncode(inp)}, nil
			case "url_decode":
				r, err := URLDecode(inp)
				if err != nil {
					return nil, err
				}
				return map[string]any{strconst.StrResult: r}, nil
			case "hash":
				algo, _ := input["algorithm"].(string)
				if algo == "" {
					algo = "sha256"
				}
				return map[string]any{strconst.StrResult: HashString(inp, algo)}, nil
			case "hmac":
				algo, _ := input["algorithm"].(string)
				key, _ := input["key"].(string)
				if algo == "" {
					algo = "sha256"
				}
				return map[string]any{strconst.StrResult: HMACString(inp, key, algo)}, nil
			case "format_duration":
				dur := time.Duration(getInt64(input["duration_ms"])) * time.Millisecond
				format, _ := input[strconst.StrFormat].(string)
				if format == "" {
					format = "short"
				}
				return map[string]any{strconst.StrResult: FormatDuration(dur, format)}, nil
			case "format_bytes":
				return map[string]any{strconst.StrResult: FormatBytes(getInt64(input["bytes"]))}, nil
			case "format_number":
				return map[string]any{strconst.StrResult: FormatNumber(getInt64(input["number"]))}, nil
			case "uuid":
				return map[string]any{strconst.StrResult: UUIDv4()}, nil
			case "timestamp":
				return map[string]any{strconst.StrResult: Timestamp()}, nil
			case "timestamp_unix":
				return map[string]any{strconst.StrResult: TimestampUnix()}, nil
			default:
				return nil, fmt.Errorf("unknown operation %q", op)
			}
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func parseMarkdownSections(raw any) []MarkdownSection {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	sections := make([]MarkdownSection, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sec := MarkdownSection{
			Heading: fmt.Sprintf("%v", obj["heading"]),
			Content: fmt.Sprintf("%v", obj[strconst.StrContent]),
			Level:   int(getInt64(obj["level"])),
		}
		if sec.Level <= 0 {
			sec.Level = 1
		}
		if cb, ok := obj["code_block"].(map[string]any); ok {
			sec.CodeBlock = &CodeBlock{
				Language: fmt.Sprintf("%v", cb["language"]),
				Code:     fmt.Sprintf("%v", cb["code"]),
			}
		}
		sections = append(sections, sec)
	}
	return sections
}
