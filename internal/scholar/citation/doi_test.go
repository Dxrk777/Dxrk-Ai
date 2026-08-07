// SPDX-License-Identifier: MIT
package citation

import "testing"

func TestValidDOI(t *testing.T) {
	tests := []struct {
		name string
		doi  string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"short prefix", "10.1000", false},
		{"missing suffix", "10.1000/", false},
		{"too short registrant", "10.1/xyz", false},
		{"invalid charset", "10.1000/foo bar", false},
		{"uppercase valid", "10.1000/ABC.123", true},
		{"lowercase valid", "10.1000/xyz123", true},
		{"arxiv valid", "10.48550/arXiv.2301.00234", true},
		{"crossref valid", "10.1145/3292500.3330701", true},
		{"nested suffix", "10.1038/nphys1505", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidDOI(tt.doi); got != tt.want {
				t.Errorf("ValidDOI(%q) = %v, want %v", tt.doi, got, tt.want)
			}
		})
	}
}

func TestExtractDOI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"url https", "https://doi.org/10.1000/abc123", "10.1000/abc123"},
		{"inline text", "ver doi 10.1000/xyz123 para más", "10.1000/xyz123"},
		{"uppercase in text", "Article DOI 10.1000/ABC.DEF", "10.1000/ABC.DEF"},
		{"no doi", "there is nothing here", ""},
		{"trailing punctuation", "doi 10.1000/abc.", "10.1000/abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractDOI(tt.in); got != tt.want {
				t.Errorf("ExtractDOI(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeDOI(t *testing.T) {
	in := "  Doi.Org/10.1000/ABC.123  "
	if got := NormalizeDOI(in); got != "doi.org/10.1000/abc.123" {
		t.Errorf("NormalizeDOI(%q) = %q", in, got)
	}
}
