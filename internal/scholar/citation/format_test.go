// SPDX-License-Identifier: MIT
package citation

import (
	"strings"
	"testing"
)

func samplePaper() Paper {
	return Paper{
		Title:   "Attention Is All You Need",
		Authors: []string{"Ashish Vaswani", "Noam Shazeer", "Niki Parmar"},
		DOI:     "10.48550/arXiv.1706.03762",
		URL:     "https://arxiv.org/abs/1706.03762",
		Year:    2017,
	}
}

func TestFormatBibTeX(t *testing.T) {
	got := FormatBibTeX(samplePaper())
	want := "@article{vaswani2017,\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("FormatBibTeX() prefix = %q, want %q", got[:min(len(got), len(want))], want)
	}
	for _, frag := range []string{
		"title = {Attention Is All You Need}",
		"author = {Vaswani, Ashish and Shazeer, Noam and Parmar, Niki}",
		"doi = {10.48550/arXiv.1706.03762}",
		"year = {2017}",
		"url = {https://arxiv.org/abs/1706.03762}",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("FormatBibTeX() missing fragment %q in:\n%s", frag, got)
		}
	}
}

func TestFormatAPA(t *testing.T) {
	got := FormatAPA(samplePaper())
	for _, frag := range []string{
		"Vaswani, A., Shazeer, N., & Parmar, N.",
		"(2017).",
		"Attention Is All You Need",
		"https://doi.org/10.48550/arXiv.1706.03762",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("FormatAPA() missing fragment %q in:\n%s", frag, got)
		}
	}

	noYear := FormatAPA(Paper{Title: "Undated", Authors: []string{"Jane Doe"}})
	if !strings.Contains(noYear, "(n.d.).") {
		t.Errorf("FormatAPA() undated paper should use (n.d.), got:\n%s", noYear)
	}

	vanDer := FormatAPA(Paper{Title: "T", Authors: []string{"Ludwig van Beethoven"}, Year: 1801})
	if !strings.Contains(vanDer, "Beethoven, L.") {
		t.Errorf("FormatAPA() should drop name particles, got:\n%s", vanDer)
	}
}

func TestFormatMLA(t *testing.T) {
	got := FormatMLA(samplePaper())
	for _, frag := range []string{
		"Vaswani, Ashish",
		"Noam Shazeer",
		"2017.",
		"Attention Is All You Need",
		"https://arxiv.org/abs/1706.03762",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("FormatMLA() missing fragment %q in:\n%s", frag, got)
		}
	}
}
