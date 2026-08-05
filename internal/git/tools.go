// SPDX-License-Identifier: MIT
package git

import (
	"context"
	"fmt"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const (
	schemaTypeObject  = strconst.StrObject
	schemaProperties  = strconst.StrProperties
	schemaTypeBoolean = "boolean"
	schemaDescription = strconst.StrDescription
	schemaTypeString  = strconst.StrString
	schemaKeyBranch   = "branch"
	schemaKeyMessage  = "message"
	schemaKeyRemote   = "remote"
	schemaRequired    = strconst.StrRequired
	descRemoteDefault = "Remoto (default: origin)"
)

type contextKey struct{}

func WithRunner(ctx context.Context, r *Runner) context.Context {
	return context.WithValue(ctx, contextKey{}, r)
}

func getRunner(ctx tools.Context) (*Runner, error) {
	if ctx.Context == nil {
		return nil, fmt.Errorf("no context available")
	}
	r, ok := ctx.Value(contextKey{}).(*Runner)
	if !ok || r == nil {
		return nil, fmt.Errorf("git not configured: set *git.Runner in context")
	}
	return r, nil
}

// RegisterTools registers git tools into the given registry.
func RegisterTools(reg *tools.Registry) error {
	toolDefs := []tools.ToolDef{
		{
			Name:        "git_status",
			Description: "Muestra el estado del repositorio git (archivos modificados, staged, untracked, conflicts).",
			InputSchema: map[string]any{
				"type":           schemaTypeObject,
				schemaProperties: map[string]any{},
			},
			Execute: func(ctx tools.Context, _ map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				status, err := r.Status(context.Background())
				if err != nil {
					return nil, err
				}
				return status, nil
			},
			IsReadOnly: ptr(true),
		},
		{
			Name:        "git_diff",
			Description: "Muestra el diff del working tree. Opcionalmente solo staged o de un archivo específico.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					"staged": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Mostrar solo cambios staged",
					},
					"path": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Ruta específica del archivo",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				staged, _ := input["staged"].(bool)
				path, _ := input["path"].(string)
				diff, err := r.Diff(context.Background(), staged, path)
				if err != nil {
					return nil, err
				}
				return diff, nil
			},
			IsReadOnly: ptr(true),
		},
		{
			Name:        "git_log",
			Description: "Muestra el historial de commits. Opcionalmente filtra por límite, author, o rama.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					"limit": map[string]any{
						"type":            strconst.StrInteger,
						schemaDescription: "Número máximo de commits (default: 10)",
					},
					schemaKeyBranch: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Nombre de la rama (default: actual)",
					},
					"all": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Mostrar todas las ramas",
					},
					"author": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Filtrar por author",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				limit, _ := input["limit"].(int)
				if limit <= 0 {
					limit = 10
				}
				all, _ := input["all"].(bool)
				author, _ := input["author"].(string)
				commits, err := r.Log(context.Background(), LogOptions{
					Limit:       limit,
					AllBranches: all,
					Author:      author,
				})
				if err != nil {
					return nil, err
				}
				return commits, nil
			},
			IsReadOnly: ptr(true),
		},
		{
			Name:        "git_commit",
			Description: "Crea un commit con los cambios staged. Si no hay archivos staged, agrega todo automáticamente.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					schemaKeyMessage: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Mensaje del commit",
					},
					"auto_add": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Auto-agregar archivos modificados (default: true)",
					},
					strconst.StrFiles: map[string]any{
						"type":            strconst.StrArray,
						schemaDescription: "Archivos específicos a incluir",
						strconst.StrItems: map[string]any{"type": schemaTypeString},
					},
				},
				schemaRequired: []string{schemaKeyMessage},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				msg, _ := input["message"].(string)
				autoAdd, _ := input["auto_add"].(bool)
				if !autoAdd {
					autoAdd = true
				}
				filesRaw, _ := input[strconst.StrFiles].([]any)

				if autoAdd || len(filesRaw) > 0 {
					if len(filesRaw) > 0 {
						files := make([]string, len(filesRaw))
						for i, f := range filesRaw {
							files[i], _ = f.(string)
						}
						if err := r.Add(context.Background(), files...); err != nil {
							return nil, err
						}
					} else {
						if err := r.Add(context.Background(), "-A"); err != nil {
							return nil, err
						}
					}
				}

				ci, err := r.Commit(context.Background(), msg, nil)
				if err != nil {
					return nil, err
				}
				return ci, nil
			},
		},
		{
			Name:        "git_branch",
			Description: "Lista las ramas del repositorio. Crea una nueva rama si se especifica un nombre.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					"all": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Mostrar ramas remotas también",
					},
					"create": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Nombre de la nueva rama a crear",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				createBranch, _ := input["create"].(string)
				if createBranch != "" {
					if err := r.Checkout(context.Background(), createBranch, true); err != nil {
						return nil, err
					}
					return map[string]any{
						schemaKeyBranch: createBranch,
						"created":       true,
					}, nil
				}

				all, _ := input["all"].(bool)
				branches, err := r.Branch(context.Background(), all)
				if err != nil {
					return nil, err
				}
				return branches, nil
			},
		},
		{
			Name:        "git_checkout",
			Description: "Cambia a una rama existente.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					schemaKeyBranch: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Nombre de la rama",
					},
				},
				schemaRequired: []string{schemaKeyBranch},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				branch, _ := input[schemaKeyBranch].(string)
				if err := r.Checkout(context.Background(), branch, false); err != nil {
					return nil, err
				}
				return map[string]any{schemaKeyBranch: branch}, nil
			},
		},
		{
			Name:        "git_push",
			Description: "Hace push de commits al remoto.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					schemaKeyRemote: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: descRemoteDefault,
					},
					schemaKeyBranch: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Rama (default: actual)",
					},
					"force": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Force push",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				remote, _ := input["remote"].(string)
				if remote == "" {
					remote = "origin"
				}
				branch, _ := input[schemaKeyBranch].(string)
				if branch == "" {
					branch, err = r.CurrentBranch(context.Background())
					if err != nil {
						return nil, err
					}
				}
				force, _ := input["force"].(bool)
				return r.Push(context.Background(), remote, branch, force)
			},
		},
		{
			Name:        "git_pull",
			Description: "Hace pull del remoto para actualizar la rama actual.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					schemaKeyRemote: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: descRemoteDefault,
					},
					schemaKeyBranch: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Rama (default: actual)",
					},
					"rebase": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Usar rebase en lugar de merge",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				remote, _ := input["remote"].(string)
				branch, _ := input[schemaKeyBranch].(string)
				rebase, _ := input["rebase"].(bool)
				return r.Pull(context.Background(), remote, branch, rebase)
			},
		},
		{
			Name:        "git_stash",
			Description: "Guarda cambios temporales (stash) o los recupera.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					"action": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Acción: push, pop, list",
						"enum":            []string{"push", "pop", "list"},
					},
					schemaKeyMessage: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Mensaje descriptivo (solo push)",
					},
					"index": map[string]any{
						"type":            strconst.StrInteger,
						schemaDescription: "Índice del stash a pop (default: 0)",
					},
				},
				schemaRequired: []string{"action"},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				action, _ := input["action"].(string)
				switch action {
				case "push":
					msg, _ := input["message"].(string)
					if err := r.Stash(context.Background(), msg); err != nil {
						return nil, err
					}
					return map[string]any{"stashed": true}, nil
				case "pop":
					idx, _ := input["index"].(int)
					if err := r.StashPop(context.Background(), idx); err != nil {
						return nil, err
					}
					return map[string]any{"popped": true}, nil
				case "list":
					stashes, err := r.StashList(context.Background())
					if err != nil {
						return nil, err
					}
					return stashes, nil
				}
				return nil, fmt.Errorf("unknown action: %s", action)
			},
		},
		{
			Name:        "git_fetch",
			Description: "Obtiene cambios del remoto sin mergear.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					schemaKeyRemote: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: descRemoteDefault,
					},
					"prune": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Eliminar refs remotas eliminadas",
					},
				},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				remote, _ := input["remote"].(string)
				prune, _ := input["prune"].(bool)
				return r.Fetch(context.Background(), remote, prune)
			},
			IsReadOnly: ptr(true),
		},
		{
			Name:        "git_create_pr",
			Description: "Crea un Pull Request en GitHub usando gh CLI.",
			InputSchema: map[string]any{
				"type": schemaTypeObject,
				schemaProperties: map[string]any{
					strconst.StrTitle: map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Título del PR",
					},
					"body": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Descripción del PR",
					},
					"base": map[string]any{
						"type":            schemaTypeString,
						schemaDescription: "Rama base (default: main)",
					},
					"draft": map[string]any{
						"type":            schemaTypeBoolean,
						schemaDescription: "Crear como draft",
					},
					"labels": map[string]any{
						"type":            strconst.StrArray,
						schemaDescription: "Labels a aplicar",
						strconst.StrItems: map[string]any{"type": schemaTypeString},
					},
				},
				schemaRequired: []string{strconst.StrTitle, "body"},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				r, err := getRunner(ctx)
				if err != nil {
					return nil, err
				}
				title, _ := input[strconst.StrTitle].(string)
				body, _ := input["body"].(string)
				base, _ := input["base"].(string)
				draft, _ := input["draft"].(bool)
				labelsRaw, _ := input["labels"].([]any)

				cfg := PRConfig{
					Title:      title,
					Body:       body,
					BaseBranch: base,
					Draft:      draft,
				}
				for _, l := range labelsRaw {
					if s, ok := l.(string); ok {
						cfg.Labels = append(cfg.Labels, s)
					}
				}

				pr, err := r.CreatePR(context.Background(), cfg)
				if err != nil {
					return nil, err
				}
				return pr, nil
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

func ptr[T any](v T) *T { return &v }

func init() {
	// Add git_remote and git_worktree as tools that return formatted strings
}
