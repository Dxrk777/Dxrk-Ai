// SPDX-License-Identifier: MIT

package citation

import (
	"regexp"
	"strings"
)

var doiPattern = regexp.MustCompile(`^10\.\d{4,9}/[-._;()/:A-Z0-9]+$`)

// ValidDOI reports whether s looks like a well-formed DOI.
//
// The structural check does not verify the check digit: many real-world registries
// assign DOIs whose suffix is not checksum-valid, so requiring one would reject
// valid identifiers.
func ValidDOI(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return doiPattern.MatchString(strings.ToUpper(s))
}

// NormalizeDOI trims whitespace and lowercases a DOI string.
func NormalizeDOI(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ExtractDOI returns the first DOI-like token found in s, or "".
// It accepts bare DOIs and URLs such as https://doi.org/10.xxxx/yyyy.
func ExtractDOI(s string) string {
	idx := strings.Index(strings.ToLower(s), "10.")
	if idx < 0 {
		return ""
	}
	candidate := s[idx:]
	for i, r := range candidate {
		if r == ' ' || r == '\t' || r == '\n' || r == ',' {
			candidate = candidate[:i]
			break
		}
	}
	candidate = strings.TrimRight(candidate, ".,;:)!?")
	if ValidDOI(candidate) {
		return candidate
	}
	return ""
}
