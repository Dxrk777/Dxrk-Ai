package webtools

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"golang.org/x/net/html"
)

// Default configuration values.
const (
	defaultTimeout      = 30 * time.Second
	defaultMaxRedirects = 10
	defaultUserAgent    = "Dxrk-Ai/1.0 WebFetcher"
	defaultMaxRPS       = 5
)

// FetchOpts configures the WebFetcher.
type FetchOpts struct {
	Timeout       time.Duration
	MaxRedirects  int
	UserAgent     string
	SkipTLSVerify bool
	ProxyURL      string
	MaxRPS        int
}

// FetchResult holds the result of a web fetch.
type FetchResult struct {
	URL         string            `json:"url"`
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	ContentType string            `json:"content_type"`
	Title       string            `json:"title"`
	FetchedAt   time.Time         `json:"fetched_at"`
	Size        int               `json:"size"`
}

// Link represents a parsed HTML link.
type Link struct {
	Text       string `json:"text"`
	Href       string `json:"href"`
	IsExternal bool   `json:"is_external"`
}

// WebFetcher performs HTTP fetches with configuration.
type WebFetcher struct {
	client       *http.Client
	timeout      time.Duration
	maxRedirects int
	userAgent    string
	mu           sync.Mutex
	lastRequest  time.Time
	maxRPS       int
}

// NewWebFetcher creates a WebFetcher with the given options.
// Zero-value FetchOpts applies sensible defaults.
func NewWebFetcher(opts FetchOpts) *WebFetcher {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = defaultMaxRedirects
	}
	if opts.UserAgent == "" {
		opts.UserAgent = defaultUserAgent
	}
	if opts.MaxRPS <= 0 {
		opts.MaxRPS = defaultMaxRPS
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: opts.SkipTLSVerify},
	}

	if opts.ProxyURL != "" {
		if proxyURL, err := url.Parse(opts.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &WebFetcher{
		client: &http.Client{
			Transport:     transport,
			Timeout:       opts.Timeout,
			CheckRedirect: makeRedirectChecker(opts.MaxRedirects),
		},
		timeout:      opts.Timeout,
		maxRedirects: opts.MaxRedirects,
		userAgent:    opts.UserAgent,
		maxRPS:       opts.MaxRPS,
	}
}

func makeRedirectChecker(max int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= max {
			return fmt.Errorf("stopped after %d redirects", max)
		}
		return nil
	}
}

func (wf *WebFetcher) rateLimit() {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	minInterval := time.Second / time.Duration(wf.maxRPS)
	elapsed := time.Since(wf.lastRequest)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	wf.lastRequest = time.Now()
}

// Fetch retrieves the content at the given URL.
func (wf *WebFetcher) Fetch(rawURL string) (*FetchResult, error) {
	wf.rateLimit()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", wf.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	resp, err := wf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readCompressedBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	contentType := resp.Header.Get("Content-Type")
	bodyStr := string(body)
	title := extractTitleFromHTML(bodyStr)

	return &FetchResult{
		URL:         resp.Request.URL.String(),
		StatusCode:  resp.StatusCode,
		Headers:     headers,
		Body:        bodyStr,
		ContentType: contentType,
		Title:       title,
		FetchedAt:   time.Now(),
		Size:        len(body),
	}, nil
}

func readCompressedBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	switch encoding {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	case "deflate":
		reader = resp.Body
	}
	return io.ReadAll(reader)
}

func extractTitleFromHTML(body string) string {
	re := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// FetchWithFormat fetches a URL and returns content in the specified format.
// Supported formats: "markdown", "text", "html", "json".
func (wf *WebFetcher) FetchWithFormat(rawURL, format string) (*FetchResult, error) {
	result, err := wf.Fetch(rawURL)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "html":
		// body is already HTML
	case "text":
		result.Body = HTMLToText(result.Body)
	case strconst.StrMarkdown:
		result.Body = HTMLToMarkdown(result.Body)
	case "json":
		result.Body = extractJSONFromHTML(result.Body)
	default:
		return nil, fmt.Errorf("unsupported format %q: use markdown, text, html, or json", format)
	}
	return result, nil
}

// HTMLToMarkdown converts HTML to a readable markdown-ish text.
func HTMLToMarkdown(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	var sb strings.Builder
	renderNodeMarkdown(doc, &sb)
	return sb.String()
}

func renderNodeMarkdown(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
		return
	}

	switch n.Data {
	case strconst.StrScript, strconst.StrStyle, "noscript":
		return
	case "br":
		sb.WriteString("\n")
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6":
		sb.WriteString("\n\n")
	case "li":
		sb.WriteString("- ")
	case "a":
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				start := sb.Len()
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					renderNodeMarkdown(c, sb)
				}
				text := strings.TrimSpace(sb.String()[start:])
				if text != "" {
					fmt.Fprintf(sb, "[%s](%s)", text, attr.Val)
				}
				return
			}
		}
	case "img":
		alt, src := "", ""
		for _, attr := range n.Attr {
			switch attr.Key {
			case "alt":
				alt = attr.Val
			case "src":
				src = attr.Val
			}
		}
		if src != "" {
			fmt.Fprintf(sb, "![%s](%s)", alt, src)
			sb.WriteString("\n")
		}
		return
	case "pre":
		sb.WriteString("\n```\n")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNodeMarkdown(c, sb)
		}
		sb.WriteString("\n```\n")
		return
	case "code":
		start := sb.Len()
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNodeMarkdown(c, sb)
		}
		inner := sb.String()[start:]
		if strings.Contains(inner, "\n") {
			return
		}
		trimmed := strings.TrimSpace(inner)
		if len(trimmed) < 80 && trimmed != "" {
			sb.Reset()
			sb.WriteString(sb.String()[:start])
			sb.WriteString("`")
			sb.WriteString(trimmed)
			sb.WriteString("` ")
		}
		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNodeMarkdown(c, sb)
	}
}

// HTMLToText extracts plain text from HTML, stripping tags.
func HTMLToText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	var sb strings.Builder
	renderNodeText(doc, &sb)
	return strings.TrimSpace(sb.String())
}

func renderNodeText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNodeText(c, sb)
		}
		return
	}
	switch n.Data {
	case strconst.StrScript, strconst.StrStyle, "noscript":
		return
	case "br", "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li":
		sb.WriteString("\n")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNodeText(c, sb)
	}
}

// ExtractContent extracts text from elements matching the given CSS-like selectors.
// Selectors are simplified: tag names, .class, and #id.
func ExtractContent(htmlContent string, selectors []string) []string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}
	var results []string
	for _, sel := range selectors {
		nodes := querySelectorSimple(doc, sel)
		for _, n := range nodes {
			text := strings.TrimSpace(extractTextContent(n))
			if text != "" {
				results = append(results, text)
			}
		}
	}
	return results
}

func querySelectorSimple(root *html.Node, selector string) []*html.Node {
	selector = strings.TrimSpace(selector)
	tag, classFilter, _ := strings.Cut(selector, ".")
	idFilter := ""

	if before, after, found := strings.Cut(selector, "#"); found {
		if classFilter != "" {
			tag = before
		}
		idFilter = after
	}

	var matches []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			match := true
			if tag != "" && tag != "*" && n.Data != tag {
				match = false
			}
			if classFilter != "" {
				classAttr := getAttr(n, "class")
				classMatch := false
				for _, c := range strings.Fields(classAttr) {
					if c == classFilter {
						classMatch = true
						break
					}
				}
				if !classMatch {
					match = false
				}
			}
			if idFilter != "" {
				if getAttr(n, "id") != idFilter {
					match = false
				}
			}
			if match {
				matches = append(matches, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return matches
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractTextContent(c))
	}
	return sb.String()
}

// ExtractLinks parses HTML and returns all links.
func ExtractLinks(htmlContent, baseURL string) []Link {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}
	base, _ := url.Parse(baseURL)

	var links []Link
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			text := strings.TrimSpace(extractTextContent(n))
			if href == "" {
				return
			}
			resolved := resolveURL(base, href)
			isExternal := false
			if resolved != "" && base != nil {
				parsed, err := url.Parse(resolved)
				if err == nil {
					isExternal = parsed.Host != "" && parsed.Host != base.Host
				}
			}
			links = append(links, Link{
				Text:       text,
				Href:       resolved,
				IsExternal: isExternal,
			})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

func resolveURL(base *url.URL, href string) string {
	if base == nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

func extractJSONFromHTML(htmlContent string) string {
	re := regexp.MustCompile(`(?s)<script[^>]*type\s*=\s*["']application/json["'][^>]*>(.*?)</script>`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)
	if len(matches) > 0 {
		parts := make([]string, 0, len(matches))
		for _, m := range matches {
			parts = append(parts, strings.TrimSpace(m[1]))
		}
		return strings.Join(parts, "\n")
	}
	return htmlContent
}
