package usertools

import (
	"fmt"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

const AskQuestionName = "ask_user_question"

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type Question struct {
	Question      string           `json:"question"`
	Options       []QuestionOption `json:"options"`
	AllowFreeform bool             `json:"allow_freeform"`
}

type AskQuestionInput struct {
	Questions []Question `json:"questions"`
}

type AskQuestionOutput struct {
	Answers map[string]string `json:"answers"`
}

type AskQuestionResult struct {
	Success bool              `json:"success"`
	Answers map[string]string `json:"answers"`
	Error   string            `json:"error,omitempty"`
}

func BuildAskQuestionTool(handler UserHandler) (tools.Tool, error) {
	return tools.Build(tools.ToolDef{
		Name:        AskQuestionName,
		Description: "Present questions with multiple choice options to the user and collect their responses. Supports free-text input when allow_freeform is true.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				"questions": map[string]any{
					"type":                  strconst.StrArray,
					strconst.StrDescription: "Questions to ask the user (1-4 questions)",
					"minItems":              1,
					"maxItems":              4,
					strconst.StrItems: map[string]any{
						"type": strconst.StrObject,
						strconst.StrProperties: map[string]any{
							"question": map[string]any{
								"type":                  strconst.StrString,
								strconst.StrDescription: "The question to present to the user",
							},
							"options": map[string]any{
								"type":                  strconst.StrArray,
								strconst.StrDescription: "Multiple choice options (2-4 options)",
								"minItems":              2,
								"maxItems":              4,
								strconst.StrItems: map[string]any{
									"type": strconst.StrObject,
									strconst.StrProperties: map[string]any{
										"label": map[string]any{
											"type":                  strconst.StrString,
											strconst.StrDescription: "Display text for this option",
										},
										strconst.StrDescription: map[string]any{
											"type":                  strconst.StrString,
											strconst.StrDescription: "Explanation of what this option means",
										},
									},
									strconst.StrRequired: []string{"label"},
								},
							},
							"allow_freeform": map[string]any{
								"type":                  "boolean",
								strconst.StrDescription: "Allow the user to type a free-text response instead of selecting an option",
							},
						},
						strconst.StrRequired: []string{"question", "options"},
					},
				},
			},
			strconst.StrRequired: []string{"questions"},
		},
		Validate: func(input map[string]any) error {
			qs, ok := input["questions"]
			if !ok {
				return fmt.Errorf("questions is required")
			}
			questions, ok := qs.([]any)
			if !ok || len(questions) == 0 {
				return fmt.Errorf("at least one question is required")
			}
			for i, q := range questions {
				qm, ok := q.(map[string]any)
				if !ok {
					return fmt.Errorf("question %d: invalid format", i)
				}
				if qm["question"] == nil || qm["question"] == "" {
					return fmt.Errorf("question %d: question text is required", i)
				}
				opts, ok := qm["options"].([]any)
				if !ok || len(opts) < 2 {
					return fmt.Errorf("question %d: at least 2 options required", i)
				}
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			questions := parseQuestions(input)
			if len(questions) == 0 {
				return AskQuestionResult{Success: false, Error: "no questions provided"}, nil
			}

			answers, err := handler.AskQuestions(ctx.Context, questions)
			if err != nil {
				//nolint:nilerr // report the error inside the tool result.
				return AskQuestionResult{
					Success: false,
					Error:   err.Error(),
				}, nil
			}

			return AskQuestionResult{
				Success: true,
				Answers: answers,
			}, nil
		},
		IsReadOnly:       boolPtr(true),
		IsConcurrentSafe: boolPtr(true),
	})
}

func parseQuestions(input map[string]any) []Question {
	qs, ok := input["questions"].([]any)
	if !ok {
		return nil
	}
	result := make([]Question, 0, len(qs))
	for _, q := range qs {
		qm, ok := q.(map[string]any)
		if !ok {
			continue
		}
		question := Question{
			Question:      fmt.Sprint(qm["question"]),
			AllowFreeform: false,
		}
		if af, ok := qm["allow_freeform"].(bool); ok {
			question.AllowFreeform = af
		}
		if opts, ok := qm["options"].([]any); ok {
			for _, o := range opts {
				om, ok := o.(map[string]any)
				if !ok {
					continue
				}
				opt := QuestionOption{
					Label: fmt.Sprint(om["label"]),
				}
				if d, ok := om[strconst.StrDescription].(string); ok {
					opt.Description = d
				}
				question.Options = append(question.Options, opt)
			}
		}
		result = append(result, question)
	}
	return result
}

func boolPtr(b bool) *bool { return &b }
