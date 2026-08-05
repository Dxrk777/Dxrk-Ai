package hooks

import (
	"regexp"
	"strings"
)

// HookMatcher provides pattern matching for hook events.
type HookMatcher struct {
	globCache  map[string]*globPattern
	regexCache map[string]*regexp.Regexp
}

type globPattern struct {
	pattern string
	regex   *regexp.Regexp
}

// NewHookMatcher creates a new hook matcher.
func NewHookMatcher() *HookMatcher {
	return &HookMatcher{
		globCache:  make(map[string]*globPattern),
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// MatchToolName checks if a tool name matches the match criteria.
func (m *HookMatcher) MatchToolName(match HookMatch, toolName string) bool {
	if match.ToolName != "" && match.ToolName != toolName {
		return false
	}
	if len(match.ToolNames) > 0 {
		found := false
		for _, n := range match.ToolNames {
			if n == toolName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// MatchPath checks if a path matches the match criteria.
func (m *HookMatcher) MatchPath(match HookMatch, path string) bool {
	if match.Path != "" && match.Path != path {
		return false
	}
	if len(match.Paths) > 0 {
		found := false
		for _, p := range match.Paths {
			if p == path {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if match.Glob != "" {
		if matched, _ := m.matchGlob(match.Glob, path); !matched {
			return false
		}
	}
	if match.Regex != "" {
		if matched, _ := m.matchRegex(match.Regex, path); !matched {
			return false
		}
	}
	return true
}

// MatchCommand checks if a command matches the match criteria.
func (m *HookMatcher) MatchCommand(match HookMatch, command string) bool {
	if match.Command != "" && match.Command != command {
		return false
	}
	if len(match.Commands) > 0 {
		found := false
		for _, c := range match.Commands {
			if c == command {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// MatchEvent checks if an event matches all criteria.
func (m *HookMatcher) MatchEvent(match HookMatch, event HookEvent) bool {
	return m.MatchToolName(match, event.ToolName) &&
		m.MatchPath(match, event.ToolName) &&
		m.MatchCommand(match, event.ToolName)
}

func (m *HookMatcher) matchGlob(pattern, name string) (bool, error) {
	if gp, ok := m.globCache[pattern]; ok {
		return gp.regex.MatchString(name), nil
	}

	regexPattern := globToRegex(pattern)
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, err
	}

	m.globCache[pattern] = &globPattern{
		pattern: pattern,
		regex:   re,
	}
	return re.MatchString(name), nil
}

func (m *HookMatcher) matchRegex(pattern, name string) (bool, error) {
	if re, ok := m.regexCache[pattern]; ok {
		return re.MatchString(name), nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}

	m.regexCache[pattern] = re
	return re.MatchString(name), nil
}

func globToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	inCharClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if !inCharClass {
				result.WriteString(".*")
			} else {
				result.WriteByte(c)
			}
		case '?':
			if !inCharClass {
				result.WriteString(".")
			} else {
				result.WriteByte(c)
			}
		case '[':
			inCharClass = true
			result.WriteByte(c)
		case ']':
			inCharClass = false
			result.WriteByte(c)
		case '\\':
			if i+1 < len(pattern) {
				result.WriteByte(c)
				result.WriteByte(pattern[i+1])
				i++
			} else {
				result.WriteByte(c)
			}
		case '.', '+', '(', ')', '^', '$', '{', '}', '|':
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
	}

	result.WriteString("$")
	return result.String()
}

// ClearCache clears the pattern caches.
func (m *HookMatcher) ClearCache() {
	m.globCache = make(map[string]*globPattern)
	m.regexCache = make(map[string]*regexp.Regexp)
}
