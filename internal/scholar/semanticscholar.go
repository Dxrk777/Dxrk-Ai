// SPDX-License-Identifier: MIT

package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const semanticscholarAPI = "https://api.semanticscholar.org/graph/v1"

const semanticscholarSearchFields = "title,abstract,year,externalIds,authors,openAccessPdf,url"

// SemanticScholarProvider queries the Semantic Scholar Graph API.
type SemanticScholarProvider struct {
	client *http.Client
}

// NewSemanticScholarProvider returns a provider backed by Semantic Scholar.
func NewSemanticScholarProvider() *SemanticScholarProvider {
	return &SemanticScholarProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Provider.
func (p *SemanticScholarProvider) Name() string {
	return "semantic_scholar"
}

// Search implements Provider.
func (p *SemanticScholarProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("fields", semanticscholarSearchFields)

	endpoint := semanticscholarAPI + "/paper/search?" + params.Encode()
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Total int       `json:"total"`
		Data  []ssPaper `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("semantic_scholar: decode search: %w", err)
	}

	papers := make([]Paper, 0, len(resp.Data))
	for _, item := range resp.Data {
		if paper := mapSSPaper(item); paper != nil {
			papers = append(papers, *paper)
		}
	}
	return papers, nil
}

// FetchByDOI implements Provider.
func (p *SemanticScholarProvider) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	endpoint := semanticscholarAPI + "/paper/DOI:" + url.PathEscape(doi) + "?fields=" + semanticscholarSearchFields
	req, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var item ssPaper
	if err := json.Unmarshal(req, &item); err != nil {
		return nil, fmt.Errorf("semantic_scholar: decode doi lookup: %w", err)
	}
	paper := mapSSPaper(item)
	if paper == nil {
		return nil, nil
	}
	return paper, nil
}

func (p *SemanticScholarProvider) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic_scholar: build request: %w", err)
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("semantic_scholar: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusTooManyRequests {
			return nil, nil
		}
		return nil, fmt.Errorf("semantic_scholar: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

type ssPaper struct {
	PaperID     string `json:"paperId"`
	Title       string `json:"title"`
	Abstract    string `json:"abstract"`
	Year        int    `json:"year"`
	ExternalIDs struct {
		DOI string `json:"DOI"`
	} `json:"externalIds"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	OpenAccessPdf struct {
		URL string `json:"url"`
	} `json:"openAccessPdf"`
	URL string `json:"url"`
}

func mapSSPaper(item ssPaper) *Paper {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil
	}
	paper := &Paper{
		Title:    title,
		Authors:  make([]string, 0, len(item.Authors)),
		DOI:      item.ExternalIDs.DOI,
		Abstract: strings.TrimSpace(item.Abstract),
		Year:     item.Year,
		PDFURL:   item.OpenAccessPdf.URL,
		Source:   "semantic_scholar",
	}
	for _, a := range item.Authors {
		name := strings.TrimSpace(a.Name)
		if name != "" {
			paper.Authors = append(paper.Authors, name)
		}
	}
	if item.URL != "" {
		paper.URL = item.URL
	} else if paper.DOI != "" {
		paper.URL = "https://doi.org/" + paper.DOI
	}
	return paper
}
