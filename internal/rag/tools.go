// SPDX-License-Identifier: MIT
package rag

import (
	"fmt"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const (
	keyDescription = strconst.StrDescription
	keyEnabled     = strconst.StrEnabled
	keyMessage     = "message"
)

// RegisterTools registers RAG tools into the given registry.
func RegisterTools(reg *tools.Registry) error {
	toolDefs := []tools.ToolDef{
		{
			Name:        "codebase_query",
			Description: "Busca código relevante en el codebase usando búsqueda semántica. Retorna fragmentos de código con ruta, línea y score de similitud.",
			InputSchema: map[string]any{
				"type": strconst.StrObject,
				strconst.StrProperties: map[string]any{
					strconst.StrQuery: map[string]any{
						"type":         strconst.StrString,
						keyDescription: "Consulta en lenguaje natural sobre lo que buscas en el código",
					},
					"max_results": map[string]any{
						"type":         strconst.StrInteger,
						keyDescription: "Máximo de resultados (default: 5)",
					},
				},
				strconst.StrRequired: []string{strconst.StrQuery},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				query, _ := input[strconst.StrQuery].(string)
				if query == "" {
					return nil, fmt.Errorf("query is required")
				}

				maxResults, _ := input["max_results"].(int)
				if maxResults <= 0 {
					maxResults = 5
				}

				rag, err := getRAGFromContext(ctx)
				if err != nil {
					return nil, err
				}

				if !rag.IsEnabled() {
					return map[string]any{
						keyEnabled: false,
						keyMessage: "RAG no está habilitado. Actívalo en dxrk.yaml (rag.enabled: true) y ejecutá el indexador primero.",
					}, nil
				}

				results, err := rag.Query(query, maxResults)
				if err != nil {
					return nil, fmt.Errorf("rag query: %w", err)
				}

				if len(results) == 0 {
					return map[string]any{
						"results": []any{},
						"message": "No se encontraron resultados. Probá indexar el codebase con codebase_reindex primero.",
					}, nil
				}

				type resultItem struct {
					FilePath  string  `json:"file_path"`
					StartLine int     `json:"start_line"`
					EndLine   int     `json:"end_line"`
					Language  string  `json:"language"`
					Text      string  `json:"text"`
					Score     float64 `json:"score"`
				}

				items := make([]resultItem, len(results))
				for i, r := range results {
					items[i] = resultItem{
						FilePath:  r.Record.Chunk.FilePath,
						StartLine: r.Record.Chunk.StartLine,
						EndLine:   r.Record.Chunk.EndLine,
						Language:  r.Record.Chunk.Language,
						Text:      r.Record.Chunk.Text,
						Score:     r.Score,
					}
				}

				return map[string]any{
					"results":  items,
					"total":    len(items),
					keyEnabled: true,
				}, nil
			},
		},
		{
			Name:        "codebase_index",
			Description: "Indexa el codebase completo: escanea archivos, genera embeddings y los almacena para búsqueda semántica.",
			InputSchema: map[string]any{
				"type": strconst.StrObject,
				strconst.StrProperties: map[string]any{
					"path": map[string]any{
						"type":         strconst.StrString,
						keyDescription: "Ruta del proyecto a indexar (default: root del proyecto)",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				rag, err := getRAGFromContext(ctx)
				if err != nil {
					return nil, err
				}

				if !rag.IsEnabled() {
					return map[string]any{
						keyEnabled: false,
						keyMessage: "RAG no está habilitado. Actívalo en dxrk.yaml (rag.enabled: true).",
					}, nil
				}

				if path, _ := input["path"].(string); path != "" {
					// Allow re-rooting for one-off indexing
					return nil, fmt.Errorf("path override not supported yet; index the project root")
				}

				stats, err := rag.Indexer.Index()
				if err != nil {
					return nil, fmt.Errorf("index: %w", err)
				}

				return map[string]any{
					"files_scanned":  stats.FilesScanned,
					"files_indexed":  stats.FilesIndexed,
					"chunks_created": stats.ChunksCreated,
					"total_vectors":  stats.TotalVectors,
					"duration_ms":    stats.DurationMs,
					"last_run":       stats.LastRun,
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

func getRAGFromContext(ctx tools.Context) (*RAG, error) {
	if ctx.Context == nil {
		return nil, fmt.Errorf("no context available")
	}
	rag, ok := ctx.Value(RAGContextKey{}).(*RAG)
	if !ok || rag == nil {
		return nil, fmt.Errorf("RAG not configured: set rag.RAG in context")
	}
	return rag, nil
}
