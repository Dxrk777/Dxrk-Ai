package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type VoiceState int

const (
	StateIdle VoiceState = iota
	StateListening
	StateProcessing
	StateSpeaking
)

func (s VoiceState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateProcessing:
		return "processing"
	case StateSpeaking:
		return "speaking"
	default:
		return strconst.StrUnknown
	}
}

type VoiceConfig struct {
	Enabled    bool
	Provider   string
	Language   string
	SampleRate int
	BufferSize int
}

type Command struct {
	Action string
	Args   []string
	Raw    string
}

type TranscriptionResult struct {
	Text       string
	Confidence float64
	IsFinal    bool
	Words      []WordTiming
}

type WordTiming struct {
	Word  string
	Start float64
	End   float64
}

type voiceSession struct {
	id        string
	startTime time.Time
	audio     []byte
	cancel    context.CancelFunc
}

type VoiceService struct {
	config   VoiceConfig
	mu       sync.Mutex
	state    VoiceState
	sessions map[string]*voiceSession
	keyTerms map[string]string
}

func NewVoiceService(config VoiceConfig) *VoiceService {
	if config.Provider == "" {
		config.Provider = strconst.StrLocal
	}
	if config.Language == "" {
		config.Language = "en-US"
	}
	if config.SampleRate <= 0 {
		config.SampleRate = 16000
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 4096
	}

	vs := &VoiceService{
		config:   config,
		state:    StateIdle,
		sessions: make(map[string]*voiceSession),
	}
	vs.initKeyTerms()
	return vs
}

func (v *VoiceService) initKeyTerms() {
	v.keyTerms = map[string]string{
		"run tests":     "Execute test suite",
		"build project": "Compile and build",
		"show errors":   "List compilation errors",
		"git status":    "Show working tree status",
		"git commit":    "Commit staged changes",
		"git push":      "Push to remote",
		"git pull":      "Pull from remote",
		"open file":     "Open file in editor",
		"search code":   "Search codebase",
		"explain code":  "Explain selected code",
		"fix lint":      "Run linter and fix issues",
		"run server":    "Start development server",
		"stop server":   "Stop development server",
		"deploy":        "Deploy to production",
		"rollback":      "Rollback last deployment",
		"help":          "Show available commands",
		"quit":          "Exit the application",
		"cancel":        "Cancel current operation",
		"retry":         "Retry last operation",
		"clear":         "Clear terminal output",
	}
}

func (v *VoiceService) StartListening(ctx context.Context) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.config.Enabled {
		return "", fmt.Errorf("voice service is disabled")
	}

	if v.state != StateIdle {
		return "", fmt.Errorf("cannot start listening: state is %s", v.state)
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	voiceID := fmt.Sprintf("voice_%d", time.Now().UnixNano())

	session := &voiceSession{
		id:        voiceID,
		startTime: time.Now(),
		audio:     make([]byte, 0, v.config.BufferSize),
		cancel:    cancel,
	}
	v.sessions[voiceID] = session
	v.state = StateListening

	fmt.Printf("[voice] started listening session %s (provider=%s lang=%s)\n", voiceID, v.config.Provider, v.config.Language)

	go v.simulateCapture(sessionCtx, voiceID)

	return voiceID, nil
}

func (v *VoiceService) simulateCapture(ctx context.Context, voiceID string) {
	ticker := time.NewTicker(time.Duration(v.config.SampleRate/v.config.BufferSize) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.mu.Lock()
			if session, ok := v.sessions[voiceID]; ok {
				chunk := make([]byte, v.config.BufferSize)
				for i := range chunk {
					chunk[i] = byte(time.Now().UnixNano() & 0xFF)
				}
				session.audio = append(session.audio, chunk...)
			}
			v.mu.Unlock()
		}
	}
}

func (v *VoiceService) StopListening(voiceID string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	session, ok := v.sessions[voiceID]
	if !ok {
		return nil, fmt.Errorf("voice session %s not found", voiceID)
	}

	if v.state != StateListening {
		return nil, fmt.Errorf("not listening: state is %s", v.state)
	}

	session.cancel()
	audio := make([]byte, len(session.audio))
	copy(audio, session.audio)

	delete(v.sessions, voiceID)
	v.state = StateIdle

	duration := time.Since(session.startTime)
	fmt.Printf("[voice] stopped listening session %s (duration=%v bytes=%d)\n", voiceID, duration, len(audio))

	return audio, nil
}

func (v *VoiceService) Transcribe(audio []byte) (string, error) {
	v.mu.Lock()
	if v.state == StateIdle {
		v.state = StateProcessing
	}
	v.mu.Unlock()
	defer func() {
		v.mu.Lock()
		if v.state == StateProcessing {
			v.state = StateIdle
		}
		v.mu.Unlock()
	}()

	if len(audio) == 0 {
		return "", fmt.Errorf("empty audio data")
	}

	if !v.config.Enabled {
		return "", fmt.Errorf("voice service is disabled")
	}

	fmt.Printf("[voice] transcribing %d bytes via %s\n", len(audio), v.config.Provider)

	result := v.transcribeWithProvider(audio)

	fmt.Printf("[voice] transcription: %q\n", result)
	return result, nil
}

func (v *VoiceService) transcribeWithProvider(audio []byte) string {
	switch v.config.Provider {
	case strconst.StrOpenai:
		return v.transcribeOpenAI(audio)
	case strconst.StrDeepgram:
		return v.transcribeDeepgram(audio)
	case strconst.StrLocal:
		return v.transcribeLocal(audio)
	default:
		return v.transcribeLocal(audio)
	}
}

func (v *VoiceService) transcribeOpenAI(audio []byte) string {
	return fmt.Sprintf("transcribed %d bytes via openai whisper", len(audio))
}

func (v *VoiceService) transcribeDeepgram(audio []byte) string {
	return fmt.Sprintf("transcribed %d bytes via deepgram nova", len(audio))
}

func (v *VoiceService) transcribeLocal(audio []byte) string {
	return fmt.Sprintf("transcribed %d bytes via local whisper.cpp", len(audio))
}

func (v *VoiceService) StreamTranscribe(ctx context.Context, audioStream <-chan []byte) (<-chan TranscriptionResult, error) {
	if !v.config.Enabled {
		return nil, fmt.Errorf("voice service is disabled")
	}

	v.mu.Lock()
	if v.state == StateIdle {
		v.state = StateProcessing
	}
	v.mu.Unlock()

	results := make(chan TranscriptionResult, 10)
	var buffer []byte

	go func() {
		defer close(results)
		defer func() {
			v.mu.Lock()
			if v.state == StateProcessing {
				v.state = StateIdle
			}
			v.mu.Unlock()
		}()

		for {
			select {
			case <-ctx.Done():
				if len(buffer) > 0 {
					text := v.transcribeWithProvider(buffer)
					results <- TranscriptionResult{
						Text:       text,
						Confidence: 0.92,
						IsFinal:    true,
					}
				}
				return
			case chunk, ok := <-audioStream:
				if !ok {
					if len(buffer) > 0 {
						text := v.transcribeWithProvider(buffer)
						results <- TranscriptionResult{
							Text:       text,
							Confidence: 0.92,
							IsFinal:    true,
						}
					}
					return
				}
				buffer = append(buffer, chunk...)
				if len(buffer) >= v.config.BufferSize {
					text := v.transcribeWithProvider(buffer)
					results <- TranscriptionResult{
						Text:       text,
						Confidence: 0.87,
						IsFinal:    false,
					}
					buffer = buffer[:0]
				}
			}
		}
	}()

	return results, nil
}

func (v *VoiceService) Speak(text string) error {
	v.mu.Lock()
	if v.state == StateIdle {
		v.state = StateSpeaking
	}
	v.mu.Unlock()
	defer func() {
		v.mu.Lock()
		if v.state == StateSpeaking {
			v.state = StateIdle
		}
		v.mu.Unlock()
	}()

	if !v.config.Enabled {
		return fmt.Errorf("voice service is disabled")
	}

	if text == "" {
		return fmt.Errorf("empty text to speak")
	}

	fmt.Printf("[voice] speaking via %s: %q\n", v.config.Provider, v.synthesizeWithProvider(text))
	return nil
}

func (v *VoiceService) synthesizeWithProvider(text string) string {
	switch v.config.Provider {
	case strconst.StrOpenai:
		return fmt.Sprintf("[openai-tts] %s", text)
	case strconst.StrDeepgram:
		return fmt.Sprintf("[deepgram-tts] %s", text)
	default:
		return fmt.Sprintf("[local-tts] %s", text)
	}
}

func (v *VoiceService) GetKeyTerms() map[string]string {
	v.mu.Lock()
	defer v.mu.Unlock()

	terms := make(map[string]string, len(v.keyTerms))
	for k, val := range v.keyTerms {
		terms[k] = val
	}
	return terms
}

func (v *VoiceService) ParseVoiceCommand(text string) Command {
	text = strings.TrimSpace(strings.ToLower(text))

	cmd := Command{
		Action: strconst.StrUnknown,
		Args:   nil,
		Raw:    text,
	}

	if text == "" {
		return cmd
	}

	parts := strings.Fields(text)
	cmd.Action = parts[0]
	if len(parts) > 1 {
		cmd.Args = parts[1:]
	}

	if known, ok := v.keyTerms[text]; ok {
		cmd.Action = parts[0]
		if len(parts) > 1 {
			cmd.Args = parts[1:]
		}
		_ = known
	}

	return cmd
}

func (v *VoiceService) IsAvailable() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.config.Enabled {
		return false
	}

	switch v.config.Provider {
	case strconst.StrOpenai, strconst.StrDeepgram, strconst.StrLocal:
		return true
	default:
		return false
	}
}

func (v *VoiceService) GetState() VoiceState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state
}

func (v *VoiceService) GetConfig() VoiceConfig {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.config
}
