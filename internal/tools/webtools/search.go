package webtools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	Snippet       string    `json:"snippet"`
	Score         float64   `json:"score"`
	PublishedDate time.Time `json:"published_date,omitempty"`
}

// SearchOpts configures a search query.
type SearchOpts struct {
	MaxResults int
	Language   string
	DateRange  string // "day", "week", "month", "year"
	SiteFilter string
	FileType   string
}

// SearchProvider is the interface that all search backends must implement.
type SearchProvider interface {
	Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
}

// WebSearcher aggregates multiple search providers.
type WebSearcher struct {
	providers  []SearchProvider
	maxResults int
}

// NewWebSearcher creates a WebSearcher with the given providers.
func NewWebSearcher(providers []SearchProvider, maxResults int) *WebSearcher {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &WebSearcher{
		providers:  providers,
		maxResults: maxResults,
	}
}

// Search queries all providers and merges results.
func (ws *WebSearcher) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	if opts.MaxResults <= 0 {
		opts.MaxResults = ws.maxResults
	}

	type providerResult struct {
		results []SearchResult
		err     error
	}

	ch := make(chan providerResult, len(ws.providers))
	for _, p := range ws.providers {
		go func(provider SearchProvider) {
			res, err := provider.Search(ctx, query, opts)
			ch <- providerResult{results: res, err: err}
		}(p)
	}

	var all []SearchResult
	var errs []string
	for range ws.providers {
		pr := <-ch
		if pr.err != nil {
			errs = append(errs, pr.err.Error())
			continue
		}
		all = append(all, pr.results...)
	}

	all = DeduplicateResults(all)
	all = RankResults(all, query)
	if len(all) > opts.MaxResults {
		all = all[:opts.MaxResults]
	}

	if len(all) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all search providers failed: %s", strings.Join(errs, "; "))
	}
	return all, nil
}

// --- Brave Search Provider ---

// BraveSearchProvider queries the Brave Search API.
type BraveSearchProvider struct {
	apiKey     string
	maxResults int
	httpClient *http.Client
}

// NewBraveSearchProvider creates a new Brave Search provider.
func NewBraveSearchProvider(apiKey string, maxResults int) *BraveSearchProvider {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &BraveSearchProvider{
		apiKey:     apiKey,
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (b *BraveSearchProvider) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	max := opts.MaxResults
	if max <= 0 {
		max = b.maxResults
	}

	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), max)
	if opts.Language != "" {
		u += "&search_lang=" + url.QueryEscape(opts.Language)
	}
	if opts.SiteFilter != "" {
		u += "&q=" + url.QueryEscape("site:"+opts.SiteFilter+" "+query)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("brave request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", b.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave search: status %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("brave decode: %w", err)
	}

	results := make([]SearchResult, len(parsed.Web.Results))
	for i, r := range parsed.Web.Results {
		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
			Score:   1.0 - float64(i)*0.05,
		}
	}
	return results, nil
}

// --- Google Custom Search Provider ---

// GoogleSearchProvider queries the Google Custom Search API.
type GoogleSearchProvider struct {
	apiKey     string
	searchCX   string
	maxResults int
	httpClient *http.Client
}

// NewGoogleSearchProvider creates a new Google Custom Search provider.
func NewGoogleSearchProvider(apiKey, searchCX string, maxResults int) *GoogleSearchProvider {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &GoogleSearchProvider{
		apiKey:     apiKey,
		searchCX:   searchCX,
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GoogleSearchProvider) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	max := opts.MaxResults
	if max <= 0 {
		max = g.maxResults
	}
	if max > 10 {
		max = 10
	}

	u := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		url.QueryEscape(g.apiKey), url.QueryEscape(g.searchCX), url.QueryEscape(query), max)
	if opts.Language != "" {
		u += "&lr=lang_" + url.QueryEscape(opts.Language)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("google request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google search: status %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("google decode: %w", err)
	}

	results := make([]SearchResult, len(parsed.Items))
	for i, item := range parsed.Items {
		results[i] = SearchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
			Score:   1.0 - float64(i)*0.08,
		}
	}
	return results, nil
}

// --- DuckDuckGo Provider ---

// DuckDuckGoProvider searches DuckDuckGo HTML (no API key needed).
type DuckDuckGoProvider struct {
	maxResults int
	httpClient *http.Client
}

// NewDuckDuckGoProvider creates a DuckDuckGo search provider.
func NewDuckDuckGoProvider(maxResults int) *DuckDuckGoProvider {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &DuckDuckGoProvider{
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DuckDuckGoProvider) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	max := opts.MaxResults
	if max <= 0 {
		max = d.maxResults
	}

	u := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, fmt.Errorf("ddg request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ddg search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ddg read: %w", err)
	}

	return parseDDGResults(string(body), max), nil
}

func parseDDGResults(htmlContent string, maxResults int) []SearchResult {
	var results []SearchResult

	titleRe := regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`(?s)<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
	urlRe := regexp.MustCompile(`(?s)<a[^>]*class="result__url"[^>]*href="([^"]*)"`)

	titles := titleRe.FindAllStringSubmatch(htmlContent, -1)
	snippets := snippetRe.FindAllStringSubmatch(htmlContent, -1)
	urls := urlRe.FindAllStringSubmatch(htmlContent, -1)

	count := len(titles)
	if len(urls) < count {
		count = len(urls)
	}
	if len(snippets) < count {
		count = len(snippets)
	}
	if maxResults > 0 && count > maxResults {
		count = maxResults
	}

	for i := 0; i < count; i++ {
		title := stripTags(strings.TrimSpace(titles[i][1]))
		snippet := stripTags(strings.TrimSpace(snippets[i][1]))
		href := strings.TrimSpace(urls[i][1])

		results = append(results, SearchResult{
			Title:   title,
			URL:     href,
			Snippet: snippet,
			Score:   1.0 - float64(i)*0.06,
		})
	}
	return results
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

// RankResults reranks results by relevance to the query using simple term frequency.
func RankResults(results []SearchResult, query string) []SearchResult {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return results
	}

	for i := range results {
		titleLower := strings.ToLower(results[i].Title)
		snippetLower := strings.ToLower(results[i].Snippet)
		score := 0.0

		for _, term := range terms {
			if strings.Contains(titleLower, term) {
				score += 0.6
			}
			if strings.Contains(snippetLower, term) {
				score += 0.4
			}
		}
		results[i].Score = score
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// DeduplicateResults removes duplicate URLs, keeping the first occurrence.
func DeduplicateResults(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var deduped []SearchResult
	for _, r := range results {
		normalized := normalizeURL(r.URL)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		deduped = append(deduped, r)
	}
	return deduped
}

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(u.Host)
	path := strings.TrimSuffix(u.Path, "/")
	return host + path
}

// FilterByQuality removes results with score below the threshold.
func FilterByQuality(results []SearchResult, minScore float64) []SearchResult {
	var filtered []SearchResult
	for _, r := range results {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
