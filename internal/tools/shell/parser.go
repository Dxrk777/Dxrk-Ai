package shell

import (
	"strings"
	"unicode"
)

type Command struct {
	Raw          string
	Name         string
	Args         []string
	HasPipe      bool
	HasRedirect  bool
	IsBackground bool
}

func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return &Command{Raw: input}
	}

	cmd := &Command{Raw: input}

	cmd.HasPipe = strings.Contains(input, "|")
	cmd.HasRedirect = strings.Contains(input, ">") || strings.Contains(input, ">>") || strings.Contains(input, "2>") || strings.Contains(input, "&>")
	cmd.IsBackground = strings.HasSuffix(input, "&") && !strings.HasSuffix(input, "&&")

	parts := splitCommand(input)
	if len(parts) == 0 {
		return cmd
	}

	name := parts[0]
	name = strings.TrimPrefix(name, "sudo ")
	name = strings.TrimPrefix(name, "env ")
	name = strings.TrimPrefix(name, "nohup ")
	cmd.Name = name
	cmd.Args = parts[1:]
	return cmd
}

func splitCommand(input string) []string {
	var parts []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	escaped := false

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		if r == '\'' && !inDouble && !inBacktick {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle && !inBacktick {
			inDouble = !inDouble
			continue
		}
		if r == '`' && !inSingle && !inDouble {
			inBacktick = !inBacktick
			continue
		}
		if unicode.IsSpace(r) && !inSingle && !inDouble && !inBacktick {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func ExtractCommands(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var commands []string
	parts := strings.Split(input, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			commands = append(commands, part)
		}
	}
	return commands
}
