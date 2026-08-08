// SPDX-License-Identifier: MIT

package scholar

import (
	"reflect"
	"testing"
)

func TestMapOpenAlexWork(t *testing.T) {
	work := openAlexWork{
		Title:           "Attention Is All You Need",
		DOI:             "https://doi.org/10.5555/3295222.3295349",
		PublicationYear: 2017,
		AbstractInverted: map[string][]int{
			"attention": {0},
			"you":       {2},
			"all":       {1},
			"need":      {3},
		},
		Authorships: []struct {
			Author struct {
				DisplayName string `json:"display_name"`
			} `json:"author"`
		}{
			{Author: struct {
				DisplayName string `json:"display_name"`
			}{DisplayName: "Ashish Vaswani"}},
		},
		PrimaryLocation: struct {
			LandingPageURL string `json:"landing_page_url"`
			PDFURL         string `json:"pdf_url"`
		}{LandingPageURL: "https://arxiv.org/abs/1706.03762", PDFURL: "https://arxiv.org/pdf/1706.03762"},
		OpenAccess: struct {
			OAURL string `json:"oa_url"`
		}{OAURL: "https://arxiv.org/pdf/1706.03762v3"},
	}

	paper := mapOpenAlexWork(work)
	if paper == nil {
		t.Fatal("expected non-nil paper")
	}
	if paper.Title != "Attention Is All You Need" {
		t.Errorf("Title = %q, want %q", paper.Title, "Attention Is All You Need")
	}
	if paper.DOI != "10.5555/3295222.3295349" {
		t.Errorf("DOI = %q, want %q", paper.DOI, "10.5555/3295222.3295349")
	}
	if paper.Year != 2017 {
		t.Errorf("Year = %d, want 2017", paper.Year)
	}
	if paper.Source != sourceOpenAlex {
		t.Errorf("Source = %q, want %q", paper.Source, sourceOpenAlex)
	}
	if paper.Abstract != "attention all you need" {
		t.Errorf("Abstract = %q, want %q", paper.Abstract, "attention all you need")
	}
	wantAuthors := []string{"Ashish Vaswani"}
	if !reflect.DeepEqual(paper.Authors, wantAuthors) {
		t.Errorf("Authors = %v, want %v", paper.Authors, wantAuthors)
	}
	if paper.URL != "https://arxiv.org/abs/1706.03762" {
		t.Errorf("URL = %q, want %q", paper.URL, "https://arxiv.org/abs/1706.03762")
	}
	if paper.PDFURL != "https://arxiv.org/pdf/1706.03762" {
		t.Errorf("PDFURL = %q, want %q", paper.PDFURL, "https://arxiv.org/pdf/1706.03762")
	}
}

func TestMapOpenAlexWorkEmptyTitle(t *testing.T) {
	if paper := mapOpenAlexWork(openAlexWork{}); paper != nil {
		t.Errorf("expected nil paper, got %+v", paper)
	}
}

func TestMapOpenAlexWorkFallbacks(t *testing.T) {
	work := openAlexWork{
		Title: "No Location",
		DOI:   "https://doi.org/10.1234/abc",
		PrimaryLocation: struct {
			LandingPageURL string `json:"landing_page_url"`
			PDFURL         string `json:"pdf_url"`
		}{},
		OpenAccess: struct {
			OAURL string `json:"oa_url"`
		}{OAURL: "https://example.com/paper.pdf"},
	}

	paper := mapOpenAlexWork(work)
	if paper == nil {
		t.Fatal("expected non-nil paper")
	}
	if paper.URL != "https://doi.org/10.1234/abc" {
		t.Errorf("URL = %q, want doi.org fallback", paper.URL)
	}
	if paper.PDFURL != "https://example.com/paper.pdf" {
		t.Errorf("PDFURL = %q, want oa_url fallback", paper.PDFURL)
	}
	if len(paper.Authors) != 0 {
		t.Errorf("Authors = %v, want empty", paper.Authors)
	}
}

func TestRebuildOpenAlexAbstractEmpty(t *testing.T) {
	if got := rebuildOpenAlexAbstract(nil); got != "" {
		t.Errorf("rebuildOpenAlexAbstract(nil) = %q, want empty", got)
	}
	if got := rebuildOpenAlexAbstract(map[string][]int{}); got != "" {
		t.Errorf("rebuildOpenAlexAbstract(empty) = %q, want empty", got)
	}
}

func TestRebuildOpenAlexAbstractOrder(t *testing.T) {
	inverted := map[string][]int{
		"world":  {4},
		"hello":  {0},
		"go":     {2},
		"the":    {3},
		"gopher": {1},
	}
	want := "hello gopher go the world"
	if got := rebuildOpenAlexAbstract(inverted); got != want {
		t.Errorf("rebuildOpenAlexAbstract = %q, want %q", got, want)
	}
}
