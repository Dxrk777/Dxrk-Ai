// SPDX-License-Identifier: MIT

package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	openalexAPI    = "https://api.openalex.org/works"
	sourceOpenAlex = "openalex"
)

// OpenAlexProvider queries the OpenAlex Works API.
type OpenAlexProvider struct {
	client *http.Client
}

// NewOpenAlexProvider returns a provider backed by OpenAlex.
func NewOpenAlexProvider() *OpenAlexProvider {
	return &OpenAlexProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Provider.
func (p *OpenAlexProvider) Name() string {
	return sourceOpenAlex
}

type openAlexResponse struct {
	Results []openAlexWork `json:"results"`
}

type openAlexWork struct {
	Title            string           `json:"display_name"`
	DOI              string           `json:"doi"`
	PublicationYear  int              `json:"publication_year"`
	AbstractInverted map[string][]int `json:"abstract_inverted_index"`
	Authorships      []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation struct {
		LandingPageURL string `json:"landing_page_url"`
		PDFURL         string `json:"pdf_url"`
	} `json:"primary_location"`
	OpenAccess struct {
		OAURL string `json:"oa_url"`
	} `json:"open_access"`
}

// Search implements Provider.
func (p *OpenAlexProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("search", query)
	params.Set("per-page", fmt.Sprintf("%d", limit))

	endpoint := openalexAPI + "?" + params.Encode()
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp openAlexResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openalex: decode search: %w", err)
	}

	papers := make([]Paper, 0, len(resp.Results))
	for _, work := range resp.Results {
		if paper := mapOpenAlexWork(work); paper != nil {
			papers = append(papers, *paper)
		}
	}
	return papers, nil
}

// FetchByDOI implements Provider.
func (p *OpenAlexProvider) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	endpoint := openalexAPI + "/doi:" + url.PathEscape(doi)
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var work openAlexWork
	if err := json.Unmarshal(body, &work); err != nil {
		return nil, fmt.Errorf("openalex: decode doi lookup: %w", err)
	}
	paper := mapOpenAlexWork(work)
	if paper == nil {
		return nil, nil
	}
	return paper, nil
}

func (p *OpenAlexProvider) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("openalex: build request: %w", err)
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0 (mailto:research@dxrk.ai)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusTooManyRequests {
			return nil, nil
		}
		return nil, fmt.Errorf("openalex: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func mapOpenAlexWork(work openAlexWork) *Paper {
	title := strings.TrimSpace(work.Title)
	if title == "" {
		return nil
	}
	paper := &Paper{
		Title:    title,
		Authors:  make([]string, 0, len(work.Authorships)),
		DOI:      strings.TrimPrefix(work.DOI, "https://doi.org/"),
		Abstract: rebuildOpenAlexAbstract(work.AbstractInverted),
		Year:     work.PublicationYear,
		Source:   sourceOpenAlex,
	}
	for _, a := range work.Authorships {
		name := strings.TrimSpace(a.Author.DisplayName)
		if name != "" {
			paper.Authors = append(paper.Authors, name)
		}
	}
	if work.PrimaryLocation.LandingPageURL != "" {
		paper.URL = work.PrimaryLocation.LandingPageURL
	} else if paper.DOI != "" {
		paper.URL = "https://doi.org/" + paper.DOI
	}
	paper.PDFURL = work.PrimaryLocation.PDFURL
	if paper.PDFURL == "" {
		paper.PDFURL = work.OpenAccess.OAURL
	}
	return paper
}

func rebuildOpenAlexAbstract(inverted map[string][]int) string {
	if len(inverted) == 0 {
		return ""
	}
	type token struct {
		pos  int
		word string
	}
	tokens := make([]token, 0, 32)
	for word, positions := range inverted {
		for _, pos := range positions {
			tokens = append(tokens, token{pos: pos, word: word})
		}
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].pos < tokens[j].pos })
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		words = append(words, t.word)
	}
	return strings.Join(words, " ")
}
