// SPDX-License-Identifier: MIT
package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const arxivAPI = "http://export.arxiv.org/api/query"

// ArxivProvider searches the arXiv API.
type ArxivProvider struct {
	client *http.Client
}

// NewArxivProvider builds an arXiv provider with a 15s HTTP client timeout.
func NewArxivProvider() *ArxivProvider {
	return &ArxivProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Provider.
func (*ArxivProvider) Name() string { return "arxiv" }

type arxivFeed struct {
	Entries []struct {
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		ID        string `json:"id"`
		Published string `json:"published"`
		Authors   []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Links []struct {
			Title string `json:"title"`
			Href  string `json:"href"`
		} `json:"link"`
	} `json:"entries"`
}

// Search implements Provider.
func (p *ArxivProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	q := url.Values{}
	q.Set("search_query", "all:"+query)
	q.Set("max_results", fmt.Sprintf("%d", limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, arxivAPI+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: status %d", resp.StatusCode)
	}
	var feed arxivFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		authors := make([]string, 0, len(e.Authors))
		for _, a := range e.Authors {
			authors = append(authors, a.Name)
		}
		year := 0
		if len(e.Published) >= 4 {
			fmt.Sscanf(e.Published[:4], "%d", &year)
		}
		papers = append(papers, Paper{
			Title:    e.Title,
			Authors:  authors,
			Abstract: strings.TrimSpace(e.Summary),
			URL:      e.ID,
			Year:     year,
			Source:   "arxiv",
		})
	}
	return papers, nil
}

// FetchByDOI implements Provider (arXiv has no DOI lookup; returns nil).
func (*ArxivProvider) FetchByDOI(context.Context, string) (*Paper, error) {
	return nil, nil
}
