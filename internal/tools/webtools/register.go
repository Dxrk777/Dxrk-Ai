package webtools

import (
	"context"
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

// Package-level defaults for tool registration.
const (
	defaultFetchTimeout = 30 * time.Second
	defaultSearchMax    = 10
	defaultCacheTTL     = 5 * time.Minute
	defaultCacheSize    = 256
)

// Shared fetcher and cache instances for tool handlers.
var (
	defaultFetcher = NewWebFetcher(FetchOpts{
		Timeout:      defaultFetchTimeout,
		MaxRedirects: 5,
		UserAgent:    defaultUserAgent,
		MaxRPS:       3,
	})
	defaultCache = NewWebCache(defaultCacheTTL, defaultCacheSize)
)

func registerWebFetch(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "web_fetch",
		Description: "Fetch content from a URL with format conversion. Supports markdown, text, html, and json output formats.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"url": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The URL to fetch",
				},
				strconst.StrFormat: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Output format: markdown, text, html, or json",
					"enum":                  []string{strconst.StrMarkdown, "text", "html", "json"},
					"default":               strconst.StrMarkdown,
				},
				strconst.StrTimeout: map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "Request timeout in seconds (default 30)",
				},
				"skip_tls_verify": map[string]any{
					"type":                  "boolean",
					strconst.StrDescription: "Skip TLS certificate verification (default false)",
				},
				"use_cache": map[string]any{
					"type":                  "boolean",
					strconst.StrDescription: "Use cached response if available (default true)",
				},
			},
			strconst.StrRequired: []string{"url"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input["url"] == nil {
				return fmt.Errorf("url is required")
			}
			u, ok := input["url"].(string)
			if !ok || u == "" {
				return fmt.Errorf("url must be a non-empty string")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return executeWebFetch(input)
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeWebFetch(input map[string]any) (any, error) {
	rawURL := input["url"].(string)
	format := strconst.StrMarkdown
	if f, ok := input[strconst.StrFormat].(string); ok && f != "" {
		format = f
	}
	useCache := true
	if c, ok := input["use_cache"].(bool); ok {
		useCache = c
	}

	if useCache {
		if entry, ok := defaultCache.Get(rawURL); ok {
			return map[string]any{
				"url":              entry.Response.URL,
				strconst.StrStatus: entry.Response.StatusCode,
				"body":             entry.Response.Body,
				strconst.StrTitle:  entry.Response.Title,
				"size":             entry.Response.Size,
				"fetched_at":       entry.FetchedAt.Format(time.RFC3339),
				"cached":           true,
			}, nil
		}
	}

	fetcher := defaultFetcher
	if timeout, ok := input[strconst.StrTimeout].(float64); ok && timeout > 0 {
		fetcher = NewWebFetcher(FetchOpts{
			Timeout:      time.Duration(timeout) * time.Second,
			MaxRedirects: 5,
			UserAgent:    defaultUserAgent,
		})
	}
	if skipTLS, ok := input["skip_tls_verify"].(bool); ok && skipTLS {
		fetcher = NewWebFetcher(FetchOpts{
			Timeout:       30 * time.Second,
			MaxRedirects:  5,
			UserAgent:     defaultUserAgent,
			SkipTLSVerify: true,
		})
	}

	result, err := fetcher.FetchWithFormat(rawURL, format)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", rawURL, err)
	}

	if useCache {
		defaultCache.Set(rawURL, result)
	}

	return map[string]any{
		"url":              result.URL,
		strconst.StrStatus: result.StatusCode,
		"body":             result.Body,
		strconst.StrTitle:  result.Title,
		"content_type":     result.ContentType,
		"size":             result.Size,
		"fetched_at":       result.FetchedAt.Format(time.RFC3339),
		"cached":           false,
	}, nil
}

func registerWebSearch(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo (default, no API key needed) or provide Brave/Google API keys for those providers.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrQuery: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Search query string",
				},
				"max_results": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "Maximum number of results (default 10)",
				},
				"language": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Language filter (e.g. en, es, fr)",
				},
				"site_filter": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Restrict to a specific site (e.g. github.com)",
				},
				"provider": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Search provider: duckduckgo (default), brave, google",
					"enum":                  []string{"duckduckgo", "brave", "google"},
				},
				"api_key": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "API key for brave or google providers",
				},
				"search_cx": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Google Custom Search engine ID (required for google provider)",
				},
			},
			strconst.StrRequired: []string{strconst.StrQuery},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrQuery] == nil {
				return fmt.Errorf("query is required")
			}
			q, ok := input[strconst.StrQuery].(string)
			if !ok || q == "" {
				return fmt.Errorf("query must be a non-empty string")
			}
			return nil
		},
		Execute:    executeWebSearch,
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeWebSearch(_ tools.Context, input map[string]any) (any, error) {
	query := input[strconst.StrQuery].(string)
	maxResults := defaultSearchMax
	if m, ok := input["max_results"].(float64); ok && m > 0 {
		maxResults = int(m)
	}

	opts := SearchOpts{MaxResults: maxResults}
	if lang, ok := input["language"].(string); ok {
		opts.Language = lang
	}
	if site, ok := input["site_filter"].(string); ok {
		opts.SiteFilter = site
	}

	provider := "duckduckgo"
	if p, ok := input["provider"].(string); ok && p != "" {
		provider = p
	}

	var searchProvider SearchProvider
	switch provider {
	case "brave":
		apiKey, _ := input["api_key"].(string)
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for brave provider")
		}
		searchProvider = NewBraveSearchProvider(apiKey, maxResults)
	case "google":
		apiKey, _ := input["api_key"].(string)
		cx, _ := input["search_cx"].(string)
		if apiKey == "" || cx == "" {
			return nil, fmt.Errorf("api_key and search_cx are required for google provider")
		}
		searchProvider = NewGoogleSearchProvider(apiKey, cx, maxResults)
	default:
		searchProvider = NewDuckDuckGoProvider(maxResults)
	}

	searcher := NewWebSearcher([]SearchProvider{searchProvider}, maxResults)
	results, err := searcher.Search(context.Background(), query, opts)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	items := make([]map[string]any, len(results))
	for i, r := range results {
		item := map[string]any{
			strconst.StrTitle: r.Title,
			"url":             r.URL,
			"snippet":         r.Snippet,
			"score":           r.Score,
		}
		if !r.PublishedDate.IsZero() {
			item["published_date"] = r.PublishedDate.Format(time.RFC3339)
		}
		items[i] = item
	}

	return map[string]any{
		"results":         items,
		strconst.StrCount: len(items),
		strconst.StrQuery: query,
	}, nil
}

func registerWebExtract(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "web_extract",
		Description: "Extract article content, metadata, code blocks, or tables from a URL. Fetches the page and parses the HTML.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"url": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The URL to extract content from",
				},
				"extract_type": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "What to extract: article, metadata, code_blocks, tables, links, or all",
					"enum":                  []string{strconst.StrArticle, "metadata", "code_blocks", "tables", "links", "all"},
					"default":               strconst.StrArticle,
				},
				"selectors": map[string]any{
					"type":                  strconst.StrArray,
					strconst.StrItems:       map[string]any{"type": strconst.StrString},
					strconst.StrDescription: "CSS-like selectors for targeted extraction (e.g. 'div.content', '#main')",
				},
			},
			strconst.StrRequired: []string{"url"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input["url"] == nil {
				return fmt.Errorf("url is required")
			}
			u, ok := input["url"].(string)
			if !ok || u == "" {
				return fmt.Errorf("url must be a non-empty string")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return executeWebExtract(input)
		},
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeWebExtract(input map[string]any) (any, error) {
	rawURL := input["url"].(string)
	extractType := strconst.StrArticle
	if et, ok := input["extract_type"].(string); ok && et != "" {
		extractType = et
	}

	result, err := defaultFetcher.Fetch(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", rawURL, err)
	}

	htmlContent := result.Body
	output := map[string]any{
		"url":              result.URL,
		strconst.StrStatus: result.StatusCode,
	}

	switch extractType {
	case strconst.StrArticle:
		article, err := ExtractArticle(htmlContent)
		if err != nil {
			return nil, fmt.Errorf("extract article: %w", err)
		}
		output[strconst.StrArticle] = article

	case "metadata":
		meta := ExtractMetadata(htmlContent)
		output["metadata"] = meta

	case "code_blocks":
		blocks := ExtractCodeBlocks(htmlContent)
		output["code_blocks"] = blocks
		output[strconst.StrCount] = len(blocks)

	case "tables":
		tables := ExtractTables(htmlContent)
		output["tables"] = tables
		output[strconst.StrCount] = len(tables)

	case "links":
		links := ExtractLinks(htmlContent, result.URL)
		output["links"] = links
		output[strconst.StrCount] = len(links)

	case "all":
		article, _ := ExtractArticle(htmlContent)
		meta := ExtractMetadata(htmlContent)
		blocks := ExtractCodeBlocks(htmlContent)
		tables := ExtractTables(htmlContent)
		links := ExtractLinks(htmlContent, result.URL)

		output[strconst.StrArticle] = article
		output["metadata"] = meta
		output["code_blocks"] = blocks
		output["tables"] = tables
		output["links"] = links
		output["code_block_count"] = len(blocks)
		output["table_count"] = len(tables)
		output["link_count"] = len(links)

	default:
		return nil, fmt.Errorf("unknown extract_type %q: use article, metadata, code_blocks, tables, links, or all", extractType)
	}

	if selectors, ok := input["selectors"].([]any); ok && len(selectors) > 0 {
		selStrings := make([]string, 0, len(selectors))
		for _, s := range selectors {
			if str, ok := s.(string); ok {
				selStrings = append(selStrings, str)
			}
		}
		if len(selStrings) > 0 {
			extracted := ExtractContent(htmlContent, selStrings)
			output["selected_content"] = extracted
		}
	}

	return output, nil
}
