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

const (
	pubmedEsearchAPI  = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi"
	pubmedEsummaryAPI = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi"
	sourcePubMed      = "pubmed"
)

// PubMedProvider queries the NCBI E-utilities API.
type PubMedProvider struct {
	client *http.Client
}

// NewPubMedProvider returns a provider backed by PubMed.
func NewPubMedProvider() *PubMedProvider {
	return &PubMedProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Provider.
func (p *PubMedProvider) Name() string {
	return sourcePubMed
}

type pubmedSearchResponse struct {
	ESearchResult struct {
		IDList []string `json:"idlist"`
	} `json:"esearchresult"`
}

type pubmedSummaryResponse struct {
	Result map[string]pubmedSummary `json:"result"`
}

type pubmedSummary struct {
	UID         string `json:"uid"`
	Title       string `json:"title"`
	PubDate     string `json:"pubdate"`
	DOI         string `json:"doi"`
	Journal     string `json:"fulljournalname"`
	Abstract    string `json:"abstract"`
	ElocationID string `json:"elocationid"`
	Authors     []struct {
		Name string `json:"name"`
	} `json:"authors"`
}

// Search implements Provider.
func (p *PubMedProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	ids, err := p.searchIDs(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return p.fetchSummaries(ctx, ids)
}

// FetchByDOI implements Provider.
func (p *PubMedProvider) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("term", doi+"[doi]")
	params.Set("retmax", "1")
	params.Set("retmode", "json")

	endpoint := pubmedEsearchAPI + "?" + params.Encode()
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var search pubmedSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("pubmed: decode doi search: %w", err)
	}
	if len(search.ESearchResult.IDList) == 0 {
		return nil, nil
	}
	papers, err := p.fetchSummaries(ctx, search.ESearchResult.IDList[:1])
	if err != nil {
		return nil, err
	}
	if len(papers) == 0 {
		return nil, nil
	}
	return &papers[0], nil
}

func (p *PubMedProvider) searchIDs(ctx context.Context, query string, limit int) ([]string, error) {
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("term", query)
	params.Set("retmax", fmt.Sprintf("%d", limit))
	params.Set("retmode", "json")

	endpoint := pubmedEsearchAPI + "?" + params.Encode()
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var search pubmedSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("pubmed: decode search: %w", err)
	}
	return search.ESearchResult.IDList, nil
}

func (p *PubMedProvider) fetchSummaries(ctx context.Context, ids []string) ([]Paper, error) {
	if len(ids) == 0 {
		return []Paper{}, nil
	}
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("id", strings.Join(ids, ","))
	params.Set("retmode", "json")

	endpoint := pubmedEsummaryAPI + "?" + params.Encode()
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp pubmedSummaryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("pubmed: decode summary: %w", err)
	}

	papers := make([]Paper, 0, len(ids))
	for _, id := range ids {
		if paper := mapPubMedSummary(resp.Result[id]); paper != nil {
			papers = append(papers, *paper)
		}
	}
	return papers, nil
}

func (p *PubMedProvider) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("pubmed: build request: %w", err)
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0 (mailto:research@dxrk.ai)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pubmed: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusTooManyRequests {
			return nil, nil
		}
		return nil, fmt.Errorf("pubmed: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func mapPubMedSummary(s pubmedSummary) *Paper {
	title := strings.TrimSpace(s.Title)
	if title == "" {
		return nil
	}
	paper := &Paper{
		Title:    title,
		Authors:  make([]string, 0, len(s.Authors)),
		DOI:      s.DOI,
		Abstract: strings.TrimSpace(s.Abstract),
		Year:     pubmedYear(s.PubDate),
		Source:   sourcePubMed,
	}
	for _, a := range s.Authors {
		if name := strings.TrimSpace(a.Name); name != "" {
			paper.Authors = append(paper.Authors, name)
		}
	}
	if s.UID != "" {
		paper.URL = "https://pubmed.ncbi.nlm.nih.gov/" + s.UID + "/"
	}
	return paper
}

func pubmedYear(pubdate string) int {
	year := 0
	if pubdate == "" {
		return year
	}
	for i := 0; i < len(pubdate); i++ {
		if pubdate[i] < '0' || pubdate[i] > '9' {
			break
		}
		year = year*10 + int(pubdate[i]-'0')
	}
	return year
}
