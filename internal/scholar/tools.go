// SPDX-License-Identifier: MIT
package scholar

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/scholar/citation"
	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

const (
	keyDescription = strconst.StrDescription
	keyEnabled     = strconst.StrEnabled
	keyMessage     = "message"
)

// ScholarContextKey is used to inject the Scholar into a tools.Context.
type ScholarContextKey struct{}

// RegisterTools registers scholarly research tools into the given registry.
func RegisterTools(reg *tools.Registry) error {
	toolDefs := []tools.ToolDef{
		{
			Name:        "scholar_search",
			Description: "Busca papers académicos (arXiv, Crossref, Semantic Scholar) y retorna título, autores, DOI, abstract y URLs.",
			InputSchema: map[string]any{
				"type": strconst.StrObject,
				strconst.StrProperties: map[string]any{
					strconst.StrQuery: map[string]any{
						"type":         strconst.StrString,
						keyDescription: "Consulta de búsqueda (términos, título o tema de investigación)",
					},
					"limit": map[string]any{
						"type":         strconst.StrInteger,
						keyDescription: "Máximo de papers por proveedor (default: 10)",
					},
					"source": map[string]any{
						"type":         strconst.StrString,
						keyDescription: "Proveedor específico: arxiv, crossref, semantic_scholar (default: todos)",
					},
				},
				strconst.StrRequired: []string{strconst.StrQuery},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				query, _ := input[strconst.StrQuery].(string)
				if strings.TrimSpace(query) == "" {
					return nil, fmt.Errorf("query is required")
				}

				limit, _ := input["limit"].(int)
				if limit <= 0 {
					limit = 10
				}

				source, _ := input["source"].(string)

				scholar, err := getScholarFromContext(ctx)
				if err != nil {
					return nil, err
				}
				if scholar == nil {
					return map[string]any{
						keyEnabled: false,
						keyMessage: "Scholar no está configurado. Inicializá internal/scholar y exponelo en el contexto de tools.",
					}, nil
				}

				results, err := scholar.Search(ctx, query, limit)
				if err != nil {
					return nil, fmt.Errorf("scholar search: %w", err)
				}

				if len(results) == 0 {
					return map[string]any{
						"results": []any{},
						"total":   0,
						"message": "No se encontraron papers. Probá con otros términos de búsqueda o una fuente específica.",
					}, nil
				}

				type resultItem struct {
					Title    string   `json:"title"`
					Authors  []string `json:"authors,omitempty"`
					DOI      string   `json:"doi,omitempty"`
					Abstract string   `json:"abstract,omitempty"`
					URL      string   `json:"url,omitempty"`
					PDFURL   string   `json:"pdf_url,omitempty"`
					Year     int      `json:"year,omitempty"`
					Source   string   `json:"source"`
				}

				items := make([]resultItem, 0, len(results))
				for _, p := range results {
					if source != "" && !strings.EqualFold(p.Source, source) {
						continue
					}
					items = append(items, resultItem(p))
				}

				return map[string]any{
					"results":  items,
					"total":    len(items),
					keyEnabled: true,
				}, nil
			},
		},
		{
			Name:        "scholar_cite",
			Description: "Genera la cita de un paper en formato BibTeX, APA y MLA a partir de su DOI.",
			InputSchema: map[string]any{
				"type": strconst.StrObject,
				strconst.StrProperties: map[string]any{
					"doi": map[string]any{
						"type":         strconst.StrString,
						keyDescription: "DOI del paper, por ejemplo 10.48550/arXiv.2301.00234",
					},
				},
				strconst.StrRequired: []string{"doi"},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				doi, _ := input["doi"].(string)
				doi = citation.NormalizeDOI(doi)
				if !citation.ValidDOI(doi) {
					return nil, fmt.Errorf("invalid DOI %q", doi)
				}

				scholar, err := getScholarFromContext(ctx)
				if err != nil {
					return nil, err
				}
				if scholar == nil {
					return map[string]any{
						keyEnabled: false,
						keyMessage: "Scholar no está configurado. Inicializá un servicio y exponelo en el contexto de tools.",
					}, nil
				}

				paper, err := scholar.FetchByDOI(ctx, doi)
				if err != nil {
					return nil, fmt.Errorf("scholar fetch: %w", err)
				}
				if paper == nil {
					return map[string]any{
						"found":    false,
						"doi":      doi,
						keyMessage: "No se encontró ningún paper para ese DOI en los proveedores configurados.",
					}, nil
				}

				c := citation.Paper{Title: paper.Title, Authors: paper.Authors, DOI: paper.DOI, Abstract: paper.Abstract, URL: paper.URL, Year: paper.Year}
				return map[string]any{
					"found":  true,
					"doi":    doi,
					"title":  paper.Title,
					"bibtex": citation.FormatBibTeX(c),
					"apa":    citation.FormatAPA(c),
					"mla":    citation.FormatMLA(c),
				}, nil
			},
		},
	}

	for _, td := range toolDefs {
		t, err := tools.Build(td)
		if err != nil {
			return fmt.Errorf("build %s: %w", td.Name, err)
		}
		if err := reg.Register(t); err != nil {
			return fmt.Errorf("register %s: %w", td.Name, err)
		}
	}
	return nil
}

func getScholarFromContext(ctx tools.Context) (*Scholar, error) {
	if ctx.Context == nil {
		return nil, fmt.Errorf("no context available")
	}
	s, ok := ctx.Value(ScholarContextKey{}).(*Scholar)
	if !ok || s == nil {
		return nil, fmt.Errorf("Scholar not configured: set scholar.Scholar in context")
	}
	return s, nil
}
