// SPDX-License-Identifier: MIT
package scholar

import "context"

// Scholar aggregates one or more providers and exposes a unified search API.
type Scholar struct {
	providers []Provider
}

// New builds a Scholar over the given providers.
func New(providers ...Provider) *Scholar {
	return &Scholar{providers: providers}
}

// Search queries every provider and merges the results.
func (s *Scholar) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	if s == nil || len(s.providers) == 0 {
		return []Paper{}, nil
	}
	papers := make([]Paper, 0, limit*len(s.providers))
	for _, p := range s.providers {
		items, err := p.Search(ctx, query, limit)
		if err != nil {
			continue
		}
		papers = append(papers, items...)
	}
	if limit > 0 && len(papers) > limit {
		papers = papers[:limit]
	}
	return papers, nil
}

// FetchByDOI returns the first non-nil paper found across providers.
func (s *Scholar) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	if s == nil {
		return nil, nil
	}
	for _, p := range s.providers {
		p, err := p.FetchByDOI(ctx, doi)
		if err != nil || p == nil {
			continue
		}
		return p, nil
	}
	return nil, nil
}
