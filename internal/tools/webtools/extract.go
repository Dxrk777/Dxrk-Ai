package webtools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"golang.org/x/net/html"
)

// Article holds extracted article content.
type Article struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	DatePublished string `json:"date_published"`
	Content       string `json:"content"`
	Summary       string `json:"summary"`
	ImageURL      string `json:"image_url"`
	SiteName      string `json:"site_name"`
}

// Metadata holds extracted HTML meta tags.
type Metadata struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Keywords      string `json:"keywords"`
	OGTitle       string `json:"og_title"`
	OGDescription string `json:"og_description"`
	OGImage       string `json:"og_image"`
	TwitterCard   string `json:"twitter_card"`
	TwitterTitle  string `json:"twitter_title"`
	TwitterDesc   string `json:"twitter_description"`
	TwitterImage  string `json:"twitter_image"`
	CanonicalURL  string `json:"canonical_url"`
	Robots        string `json:"robots"`
	ContentType   string `json:"content_type"`
	Author        string `json:"author"`
}

// CodeBlock represents an extracted code snippet.
type CodeBlock struct {
	Language  string `json:"language"`
	Code      string `json:"code"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
}

// ExtractArticle extracts the main article content from HTML.
func ExtractArticle(htmlContent string) (*Article, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	meta := ExtractMetadata(htmlContent)
	article := &Article{
		Title:    meta.Title,
		SiteName: meta.Description,
	}

	articleNode := findArticleNode(doc)
	if articleNode == nil {
		articleNode = findLargestTextNode(doc)
	}

	if articleNode != nil {
		article.Content = extractReadableText(articleNode)
		article.Content = cleanText(article.Content)
	}

	if article.Title == "" {
		article.Title = extractTitleFromHTML(htmlContent)
	}
	article.Author = meta.Author
	article.ImageURL = meta.OGImage
	article.Summary = SummarizeText(article.Content, 3)

	return article, nil
}

func findArticleNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode {
		class := getAttr(n, "class")
		id := getAttr(n, "id")

		articleSelectors := []string{strconst.StrArticle, "post-content", "entry-content", "article-content", "main-content", "story-body"}
		for _, sel := range articleSelectors {
			if n.Data == strconst.StrArticle || strings.Contains(class, sel) || strings.Contains(id, sel) {
				return n
			}
		}

		if n.Data == "main" || strings.Contains(class, "main") {
			return n
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findArticleNode(c); found != nil {
			return found
		}
	}
	return nil
}

func findLargestTextNode(n *html.Node) *html.Node {
	var best *html.Node
	bestLen := 0

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := node.Data
			if tag == strconst.StrScript || tag == strconst.StrStyle || tag == "nav" || tag == "header" || tag == "footer" {
				return
			}
			text := extractReadableText(node)
			if len(text) > bestLen {
				bestLen = len(text)
				best = node
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return best
}

func extractReadableText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractReadableText(c))
	}
	return sb.String()
}

func cleanText(text string) string {
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	return text
}

// ExtractMetadata extracts meta tags from HTML.
func ExtractMetadata(htmlContent string) *Metadata {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return &Metadata{}
	}

	meta := &Metadata{}
	meta.Title = extractTitleFromHTML(htmlContent)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			name := strings.ToLower(getAttr(n, "name"))
			property := strings.ToLower(getAttr(n, "property"))
			content := getAttr(n, strconst.StrContent)

			switch {
			case name == strconst.StrDescription:
				meta.Description = content
			case name == "keywords":
				meta.Keywords = content
			case name == "author":
				meta.Author = content
			case name == "robots":
				meta.Robots = content
			case name == "twitter:card":
				meta.TwitterCard = content
			case name == "twitter:title":
				meta.TwitterTitle = content
			case name == "twitter:description":
				meta.TwitterDesc = content
			case name == "twitter:image":
				meta.TwitterImage = content
			case property == "og:title":
				meta.OGTitle = content
			case property == "og:description":
				meta.OGDescription = content
			case property == "og:image":
				meta.OGImage = content
			}
		}

		if n.Type == html.ElementNode && n.Data == "link" {
			rel := strings.ToLower(getAttr(n, "rel"))
			if rel == "canonical" {
				meta.CanonicalURL = getAttr(n, "href")
			}
		}

		if n.Type == html.ElementNode && n.Data == strconst.StrTitle {
			if meta.Title == "" {
				meta.Title = extractTextContent(n)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if meta.OGTitle != "" && meta.Title == "" {
		meta.Title = meta.OGTitle
	}
	return meta
}

// ExtractCodeBlocks extracts code snippets from HTML.
func ExtractCodeBlocks(htmlContent string) []CodeBlock {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}

	var blocks []CodeBlock
	lineCounter := 1

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "pre" || n.Data == "code") {
			lang := detectCodeLanguage(n)
			code := extractReadableText(n)
			code = strings.TrimSpace(code)
			if code != "" {
				lines := strings.Split(code, "\n")
				start := lineCounter
				end := start + len(lines) - 1
				blocks = append(blocks, CodeBlock{
					Language:  lang,
					Code:      code,
					LineStart: start,
					LineEnd:   end,
				})
				lineCounter = end + 1
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return blocks
}

func detectCodeLanguage(n *html.Node) string {
	class := getAttr(n, "class")
	langPatterns := map[string]string{
		"language-": "", "lang-": "", "highlight-": "",
	}
	for prefix := range langPatterns {
		if idx := strings.Index(class, prefix); idx >= 0 {
			rest := class[idx+len(prefix):]
			end := strings.IndexAny(rest, " \t\n")
			if end > 0 {
				return rest[:end]
			}
			return rest
		}
	}

	if n.Type == html.ElementNode && n.Data == "pre" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "code" {
				return detectCodeLanguage(c)
			}
		}
	}
	return "text"
}

// ExtractTables extracts HTML tables into 2D string slices.
func ExtractTables(htmlContent string) [][]string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}

	var tables [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			table := extractTableData(n)
			if len(table) > 0 {
				tables = append(tables, table...)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

func extractTableData(n *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "tr" {
			var cells []string
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cellText := strings.TrimSpace(extractReadableText(c))
					cells = append(cells, cellText)
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return rows
}

// SummarizeText performs extractive summarization by selecting the most important sentences.
func SummarizeText(text string, maxSentences int) string {
	if maxSentences <= 0 {
		maxSentences = 3
	}
	sentences := splitSentences(text)
	if len(sentences) <= maxSentences {
		return strings.Join(sentences, " ")
	}

	type scored struct {
		text  string
		score float64
	}
	var scoredSentences []scored

	stopWords := buildStopWords()
	wordFreq := buildWordFrequency(text, stopWords)

	for i, sent := range sentences {
		s := scored{text: sent}
		words := strings.Fields(strings.ToLower(sent))
		for _, w := range words {
			if stopWords[w] {
				continue
			}
			s.score += wordFreq[w]
		}
		if i == 0 || i == len(sentences)-1 {
			s.score += 0.3
		}
		scoredSentences = append(scoredSentences, s)
	}

	sort.Slice(scoredSentences, func(i, j int) bool {
		return scoredSentences[i].score > scoredSentences[j].score
	})

	type indexScore struct {
		idx   int
		score float64
	}
	var selected []indexScore
	for i, s := range scoredSentences {
		if i >= maxSentences {
			break
		}
		for j, sent := range sentences {
			if sent == s.text {
				selected = append(selected, indexScore{idx: j, score: s.score})
				break
			}
		}
	}

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].idx < selected[j].idx
	})

	var result []string
	for _, s := range selected {
		result = append(result, sentences[s.idx])
	}
	return strings.Join(result, " ")
}

func splitSentences(text string) []string {
	re := regexp.MustCompile(`[.!?]+\s+`)
	parts := re.Split(text, -1)
	var sentences []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			sentences = append(sentences, p)
		}
	}
	return sentences
}

func buildStopWords() map[string]bool {
	words := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to",
		"for", "of", "with", "by", "from", "is", "are", "was", "were",
		"be", "been", "being", "have", "has", "had", "do", "does", "did",
		"will", "would", "could", "should", "may", "might", "shall",
		"can", "this", "that", "these", "those", "it", "its", "he",
		"she", "they", "we", "you", "i", "me", "my", "our", "your",
		"his", "her", "their", "not", "no", "if", "then", "else",
		"when", "up", "out", "so", "than", "too", "very", "just",
		"about", "into", "through", "during", "before", "after",
		"above", "below", "between", "while", "because", "as",
	}
	stop := make(map[string]bool)
	for _, w := range words {
		stop[w] = true
	}
	return stop
}

func buildWordFrequency(text string, stopWords map[string]bool) map[string]float64 {
	words := strings.Fields(strings.ToLower(text))
	freq := make(map[string]float64)
	for _, w := range words {
		w = strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if w == "" || stopWords[w] {
			continue
		}
		freq[w]++
	}
	maxFreq := 0.0
	for _, f := range freq {
		if f > maxFreq {
			maxFreq = f
		}
	}
	if maxFreq > 0 {
		for w := range freq {
			freq[w] /= maxFreq
		}
	}
	return freq
}

// ExtractJSON extracts JSON objects and arrays from text.
func ExtractJSON(text string) ([]string, error) {
	var results []string
	re := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}|\[[^\[\]]*(?:\[[^\[\]]*\][^\[\]]*)*\]`)

	matches := re.FindAllString(text, -1)
	for _, m := range matches {
		var v any
		if err := json.Unmarshal([]byte(m), &v); err == nil {
			pretty, err := json.MarshalIndent(v, "", "  ")
			if err == nil {
				results = append(results, string(pretty))
			} else {
				results = append(results, m)
			}
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no valid JSON found in text")
	}
	return results, nil
}

// ExtractJSONFromText is an alias for ExtractJSON.
func ExtractJSONFromText(text string) ([]string, error) {
	return ExtractJSON(text)
}
