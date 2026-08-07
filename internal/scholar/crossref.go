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

const crossrefAPI = "https://api.crossref.org/works"

// CrossrefProvider searches the Crossref REST API.
type CrossrefProvider struct {
	client *http.Client
}

// NewCrossrefProvider builds a Crossref provider with a 15s HTTP client timeout.
func NewCrossrefProvider() *CrossrefProvider {
	return &CrossrefProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Provider.
func (*CrossrefProvider) Name() string { return "crossref" }

type crossrefResponse struct {
	Message struct {
		Items []struct {
			Title  []string `json:"title"`
			Author []struct {
				Family string `json:"family"`
				Given  string `json:"given"`
			} `json:"author"`
			DOI      string `json:"DOI"`
			Abstract string `json:"abstract"`
			URL      string `json:"URL"`
			Link     []struct {
				URL string `json:"URL"`
			} `json:"link"`
			Issued struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"issued"`
			ContainerTitle []string `json:"container-title"`
		} `json:"items"`
	} `json:"message"`
}

// Search implements Provider.
func (p *CrossrefProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("rows", fmt.Sprintf("%d", limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crossrefAPI+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0 (mailto:research@dxrk.ai)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crossref: status %d", resp.StatusCode)
	}
	var cr crossrefResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(cr.Message.Items))
	for _, it := range cr.Message.Items {
		authors := make([]string, 0, len(it.Author))
		for _, a := range it.Author {
			name := strings.TrimSpace(a.Given + " " + a.Family)
			if name != "" {
				authors = append(authors, name)
			}
		}
		year := 0
		if len(it.Issued.DateParts) > 0 && len(it.Issued.DateParts[0]) > 0 {
			year = it.Issued.DateParts[0][0]
		}
		title := ""
		if len(it.Title) > 0 {
			title = it.Title[0]
		}
		pdfURL := ""
		for _, l := range it.Link {
			if l.URL != "" {
				pdfURL = l.URL
				break
			}
		}
		papers = append(papers, Paper{
			Title:    title,
			Authors:  authors,
			DOI:      it.DOI,
			Abstract: strings.TrimSpace(it.Abstract),
			URL:      it.URL,
			PDFURL:   pdfURL,
			Year:     year,
			Source:   "crossref",
		})
	}
	return papers, nil
}

// FetchByDOI implements Provider.
func (p *CrossrefProvider) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crossrefAPI+"/"+url.PathEscape(doi), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dxrk-scholar/1.0 (mailto:research@dxrk.ai)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crossref: status %d", resp.StatusCode)
	}
	var single struct {
		Message struct {
			Title  []string `json:"title"`
			Author []struct {
				Family string `json:"family"`
				Given  string `json:"given"`
			} `json:"author"`
			DOI      string `json:"DOI"`
			Abstract string `json:"abstract"`
			URL      string `json:"URL"`
			Issued   struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"issued"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&single); err != nil {
		return nil, err
	}
	m := single.Message
	title := ""
	if len(m.Title) > 0 {
		title = m.Title[0]
	}
	year := 0
	if len(m.Issued.DateParts) > 0 && len(m.Issued.DateParts[0]) > 0 {
		year = m.Issued.DateParts[0][0]
	}
	authors := make([]string, 0, len(m.Author))
	for _, a := range m.Author {
		name := strings.TrimSpace(a.Given + " " + a.Family)
		if name != "" {
			authors = append(authors, name)
		}
	}
	return &Paper{
		Title:    title,
		Authors:  authors,
		DOI:      m.DOI,
		Abstract: strings.TrimSpace(m.Abstract),
		URL:      m.URL,
		Year:     year,
		Source:   "crossref",
	}, nil
}
