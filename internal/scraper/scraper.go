// SPDX-License-Identifier: MIT
package scraper

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gocolly/colly/v2"
)

type Result struct {
	URL     string
	Title   string
	Content string
	Links   []string
}

func Scrape(url string, timeout time.Duration) (*Result, error) {
	c := colly.NewCollector(
		colly.AllowedDomains(extractDomain(url)),
		colly.UserAgent("Dxrk/1.0"),
	)

	c.SetRequestTimeout(timeout)

	res := &Result{URL: url}

	c.OnHTML("title", func(e *colly.HTMLElement) {
		res.Title = e.Text
	})

	c.OnHTML("body", func(e *colly.HTMLElement) {
		res.Content = e.Text
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if link != "" {
			res.Links = append(res.Links, link)
		}
	})

	if err := c.Visit(url); err != nil {
		return nil, fmt.Errorf("scrape %s: %w", url, err)
	}

	return res, nil
}

func extractDomain(rawURL string) string {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return ""
	}
	return req.URL.Host
}
