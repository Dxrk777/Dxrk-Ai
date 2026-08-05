package usertools

import "context"

type UserHandler interface {
	AskQuestions(ctx context.Context, questions []Question) (map[string]string, error)
	SendMessage(ctx context.Context, message string, files []AttachmentInfo) error
}
