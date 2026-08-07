// SPDX-License-Identifier: MIT
package scholar

import "context"

// Paper represents a scholarly publication returned by a provider.
type Paper struct {
	Title    string   `json:"title"`
	Authors  []string `json:"authors,omitempty"`
	DOI      string   `json:"doi,omitempty"`
	Abstract string   `json:"abstract,omitempty"`
	URL      string   `json:"url,omitempty"`
	PDFURL   string   `json:"pdf_url,omitempty"`
	Year     int      `json:"year,omitempty"`
	Source   string   `json:"source"`
}

// Provider abstracts a scholarly search backend (arXiv, Crossref, Semantic Scholar).
type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Paper, error)
	FetchByDOI(ctx context.Context, doi string) (*Paper, error)
}
