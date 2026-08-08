// SPDX-License-Identifier: MIT

package scholar

import (
	"reflect"
	"testing"
)

func TestMapPubMedSummary(t *testing.T) {
	s := pubmedSummary{
		UID:      "36303072",
		Title:    "A Study of Something",
		PubDate:  "2023 Jan 15",
		DOI:      "10.1234/study",
		Abstract: "Some abstract text.",
		Authors: []struct {
			Name string `json:"name"`
		}{
			{Name: "Jane Doe"},
			{Name: "John Smith"},
			{Name: "  "},
		},
	}

	paper := mapPubMedSummary(s)
	if paper == nil {
		t.Fatal("expected non-nil paper")
	}
	if paper.Title != "A Study of Something" {
		t.Errorf("Title = %q, want %q", paper.Title, "A Study of Something")
	}
	if paper.DOI != "10.1234/study" {
		t.Errorf("DOI = %q, want %q", paper.DOI, "10.1234/study")
	}
	if paper.Year != 2023 {
		t.Errorf("Year = %d, want 2023", paper.Year)
	}
	if paper.Source != sourcePubMed {
		t.Errorf("Source = %q, want %q", paper.Source, sourcePubMed)
	}
	if paper.URL != "https://pubmed.ncbi.nlm.nih.gov/36303072/" {
		t.Errorf("URL = %q, want pubmed url", paper.URL)
	}
	wantAuthors := []string{"Jane Doe", "John Smith"}
	if !reflect.DeepEqual(paper.Authors, wantAuthors) {
		t.Errorf("Authors = %v, want %v", paper.Authors, wantAuthors)
	}
}

func TestMapPubMedSummaryEmptyTitle(t *testing.T) {
	if paper := mapPubMedSummary(pubmedSummary{}); paper != nil {
		t.Errorf("expected nil paper, got %+v", paper)
	}
}

func TestPubmedYear(t *testing.T) {
	cases := map[string]int{
		"2023 Jan 15":  2023,
		"2022":         2022,
		"2023 Jul-Aug": 2023,
		"":             0,
		"not a date":   0,
		"20A":          20,
	}
	for in, want := range cases {
		if got := pubmedYear(in); got != want {
			t.Errorf("pubmedYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPubMedFetchSummariesEmpty(t *testing.T) {
	p := NewPubMedProvider()
	papers, err := p.fetchSummaries(t.Context(), nil)
	if err != nil {
		t.Fatalf("fetchSummaries(nil) error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("fetchSummaries(nil) = %v, want empty", papers)
	}
}
