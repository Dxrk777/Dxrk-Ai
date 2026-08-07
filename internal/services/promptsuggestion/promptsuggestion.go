package promptsuggestion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type SuggestionState int

const (
	StateIdle SuggestionState = iota
	StatePending
	StateStarted
	StateDone
	StateStopped
)

func (s SuggestionState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StatePending:
		return strconst.StrPending
	case StateStarted:
		return "started"
	case StateDone:
		return "done"
	case StateStopped:
		return "stopped"
	default:
		return strconst.StrUnknown
	}
}

type Suggestion struct {
	ID          string
	Text        string
	State       SuggestionState
	CreatedAt   time.Time
	CompletedAt time.Time
	Error       error
}

func (s *Suggestion) String() string {
	return s.Text
}

func (s *Suggestion) IsTerminal() bool {
	return s.State == StateDone || s.State == StateStopped
}

type PromptSuggestionConfig struct {
	Enabled        bool
	MaxSuggestions int
	TimeoutSeconds int
}

type PromptSuggestionService struct {
	config            PromptSuggestionConfig
	mu                sync.Mutex
	suggestions       []Suggestion
	currentSuggestion *Suggestion
	idCounter         int
}

func NewPromptSuggestionService(config PromptSuggestionConfig) *PromptSuggestionService {
	if config.MaxSuggestions <= 0 {
		config.MaxSuggestions = 3
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 10
	}
	return &PromptSuggestionService{
		config: config,
	}
}

func (s *PromptSuggestionService) RequestSuggestion(ctx context.Context, prompt string) (*Suggestion, error) {
	s.mu.Lock()
	if !s.config.Enabled {
		s.mu.Unlock()
		return nil, fmt.Errorf("prompt suggestion is disabled")
	}

	if s.currentSuggestion != nil && !s.currentSuggestion.IsTerminal() {
		s.mu.Unlock()
		return nil, fmt.Errorf("a suggestion is already in progress")
	}

	s.idCounter++
	suggestion := Suggestion{
		ID:        fmt.Sprintf("suggestion-%d", s.idCounter),
		Text:      prompt,
		State:     StatePending,
		CreatedAt: time.Now(),
	}

	s.suggestions = append(s.suggestions, suggestion)
	s.currentSuggestion = &s.suggestions[len(s.suggestions)-1]
	ref := s.currentSuggestion
	s.mu.Unlock()

	ref.State = StateStarted

	go s.runSuggestion(ctx, ref)

	return ref, nil
}

func (s *PromptSuggestionService) runSuggestion(ctx context.Context, suggestion *Suggestion) {
	timeout := time.Duration(s.config.TimeoutSeconds) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		suggestion.State = StateStopped
		suggestion.Error = ctx.Err()
		suggestion.CompletedAt = time.Now()
		s.mu.Unlock()
	case <-timer.C:
		s.mu.Lock()
		suggestion.State = StateDone
		suggestion.CompletedAt = time.Now()
		s.mu.Unlock()
	}
}

func (s *PromptSuggestionService) GetSuggestions() []Suggestion {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Suggestion, len(s.suggestions))
	copy(result, s.suggestions)
	return result
}

func (s *PromptSuggestionService) StopSuggestion(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.suggestions {
		if s.suggestions[i].ID == id {
			if s.suggestions[i].IsTerminal() {
				return fmt.Errorf("suggestion %s is already in terminal state", id)
			}
			s.suggestions[i].State = StateStopped
			s.suggestions[i].CompletedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("suggestion %s not found", id)
}

func (s *PromptSuggestionService) ClearSuggestions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.suggestions = nil
	s.currentSuggestion = nil
}

func (s *PromptSuggestionService) NextSuggestion() *Suggestion {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.suggestions {
		if s.suggestions[i].State == StateDone {
			return &s.suggestions[i]
		}
	}
	return nil
}
