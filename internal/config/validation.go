// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ---- Validation Types ----

// Severity indicates the severity of a validation error.
type Severity string

const (
	SeverityError   Severity = strconst.StrError
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Validator validates a HierarchicalConfig and returns errors.
type Validator interface {
	Validate(config *HierarchicalConfig) []ConfigError
}

// ---- Built-in Validators ----

// ModelValidator checks model configuration.
type ModelValidator struct{}

func (v ModelValidator) Validate(cfg *HierarchicalConfig) []ConfigError {
	var errs []ConfigError

	if cfg.Model.Provider == "" {
		errs = append(errs, ConfigError{
			Path:       "model.provider",
			Message:    "provider must not be empty",
			Severity:   string(SeverityError),
			Suggestion: "set provider to one of: claude, openai, gemini, ollama",
		})
	}

	if cfg.Model.ModelName == "" {
		errs = append(errs, ConfigError{
			Path:       "model.model_name",
			Message:    "model name must not be empty",
			Severity:   string(SeverityError),
			Suggestion: "set a valid model name for your provider",
		})
	}

	if cfg.Model.MaxTokens < 0 {
		errs = append(errs, ConfigError{
			Path:       "model.max_tokens",
			Message:    fmt.Sprintf("max_tokens must be non-negative, got %d", cfg.Model.MaxTokens),
			Severity:   string(SeverityError),
			Suggestion: "set max_tokens to a value between 0 and 1000000",
		})
	}

	if cfg.Model.MaxTokens > 1000000 {
		errs = append(errs, ConfigError{
			Path:       "model.max_tokens",
			Message:    fmt.Sprintf("max_tokens exceeds maximum (1000000), got %d", cfg.Model.MaxTokens),
			Severity:   string(SeverityWarning),
			Suggestion: "most models support up to 128k tokens",
		})
	}

	if cfg.Model.Temperature < 0 || cfg.Model.Temperature > 2.0 {
		errs = append(errs, ConfigError{
			Path:       "model.temperature",
			Message:    fmt.Sprintf("temperature must be 0.0-2.0, got %.2f", cfg.Model.Temperature),
			Severity:   string(SeverityError),
			Suggestion: "use 0.7 for balanced output, 0.0 for deterministic",
		})
	}

	if cfg.Model.TopP < 0 || cfg.Model.TopP > 1.0 {
		errs = append(errs, ConfigError{
			Path:       "model.top_p",
			Message:    fmt.Sprintf("top_p must be 0.0-1.0, got %.2f", cfg.Model.TopP),
			Severity:   string(SeverityError),
			Suggestion: "use 0.9 for most use cases",
		})
	}

	return errs
}

// APIValidator checks API configuration.
type APIValidator struct{}

func (v APIValidator) Validate(cfg *HierarchicalConfig) []ConfigError {
	var errs []ConfigError

	if cfg.API.BaseURL == "" {
		errs = append(errs, ConfigError{
			Path:       "api.base_url",
			Message:    "base_url must not be empty",
			Severity:   string(SeverityError),
			Suggestion: "set the API endpoint URL",
		})
	} else if _, err := url.ParseRequestURI(cfg.API.BaseURL); err != nil {
		errs = append(errs, ConfigError{
			Path:       "api.base_url",
			Message:    fmt.Sprintf("invalid URL format: %v", err),
			Severity:   string(SeverityError),
			Suggestion: "use format: https://api.example.com",
		})
	}

	if cfg.API.Timeout < 0 {
		errs = append(errs, ConfigError{
			Path:       "api.timeout",
			Message:    fmt.Sprintf("timeout must be non-negative, got %d", cfg.API.Timeout),
			Severity:   string(SeverityError),
			Suggestion: "use 30 seconds as a reasonable default",
		})
	}

	if cfg.API.Timeout > 600 {
		errs = append(errs, ConfigError{
			Path:       "api.timeout",
			Message:    fmt.Sprintf("timeout exceeds maximum (600s), got %d", cfg.API.Timeout),
			Severity:   string(SeverityWarning),
			Suggestion: "consider reducing timeout to avoid hanging",
		})
	}

	if cfg.API.Retries < 0 {
		errs = append(errs, ConfigError{
			Path:       "api.retries",
			Message:    fmt.Sprintf("retries must be non-negative, got %d", cfg.API.Retries),
			Severity:   string(SeverityError),
			Suggestion: "use 3 retries as a reasonable default",
		})
	}

	if cfg.API.Retries > 10 {
		errs = append(errs, ConfigError{
			Path:       "api.retries",
			Message:    fmt.Sprintf("retries exceeds maximum (10), got %d", cfg.API.Retries),
			Severity:   string(SeverityWarning),
			Suggestion: "excessive retries may cause rate limiting",
		})
	}

	if cfg.API.RateLimit < 0 {
		errs = append(errs, ConfigError{
			Path:     "api.rate_limit",
			Message:  fmt.Sprintf("rate_limit must be non-negative, got %d", cfg.API.RateLimit),
			Severity: string(SeverityError),
		})
	}

	return errs
}

// PathValidator checks that referenced file paths exist.
type PathValidator struct{}

func (v PathValidator) Validate(cfg *HierarchicalConfig) []ConfigError {
	var errs []ConfigError

	paths := []struct {
		path string
		name string
	}{
		{cfg.Auth.TokenPath, "auth.token_path"},
	}

	for _, p := range paths {
		if p.path == "" {
			continue
		}
		expanded := expandPath(p.path)
		if _, err := os.Stat(expanded); err != nil {
			if os.IsNotExist(err) {
				errs = append(errs, ConfigError{
					Path:       p.name,
					Message:    fmt.Sprintf("path does not exist: %s", expanded),
					Severity:   string(SeverityWarning),
					Suggestion: "create the directory or update the path",
				})
			} else {
				errs = append(errs, ConfigError{
					Path:     p.name,
					Message:  fmt.Sprintf("cannot access path: %v", err),
					Severity: string(SeverityWarning),
				})
			}
		}
	}

	return errs
}

// PortValidator checks port-related settings.
type PortValidator struct{}

func (v PortValidator) Validate(cfg *HierarchicalConfig) []ConfigError {
	var errs []ConfigError

	// Check session settings for reasonable values
	if cfg.Session.MaxHistory < 0 {
		errs = append(errs, ConfigError{
			Path:     "session.max_history",
			Message:  fmt.Sprintf("max_history must be non-negative, got %d", cfg.Session.MaxHistory),
			Severity: string(SeverityError),
		})
	}

	if cfg.Session.ArchiveAfter < 0 {
		errs = append(errs, ConfigError{
			Path:     "session.archive_after",
			Message:  fmt.Sprintf("archive_after must be non-negative, got %d", cfg.Session.ArchiveAfter),
			Severity: string(SeverityError),
		})
	}

	if cfg.Tools.MaxConcurrent < 0 {
		errs = append(errs, ConfigError{
			Path:     "tools.max_concurrent",
			Message:  fmt.Sprintf("max_concurrent must be non-negative, got %d", cfg.Tools.MaxConcurrent),
			Severity: string(SeverityError),
		})
	}

	if cfg.Tools.MaxConcurrent > 100 {
		errs = append(errs, ConfigError{
			Path:       "tools.max_concurrent",
			Message:    fmt.Sprintf("max_concurrent exceeds safe limit (100), got %d", cfg.Tools.MaxConcurrent),
			Severity:   string(SeverityWarning),
			Suggestion: "high concurrency may cause resource exhaustion",
		})
	}

	return errs
}

// CompositeValidator runs multiple validators and aggregates results.
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a validator that runs all provided validators.
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{validators: validators}
}

func (v *CompositeValidator) Validate(cfg *HierarchicalConfig) []ConfigError {
	allErrs := make([]ConfigError, 0, len(v.validators))
	for _, validator := range v.validators {
		allErrs = append(allErrs, validator.Validate(cfg)...)
	}
	return allErrs
}

// ---- Convenience Functions ----

// ValidateConfig runs the default validation pipeline on a config.
func ValidateConfig(cfg *HierarchicalConfig) []ConfigError {
	validator := NewCompositeValidator(
		ModelValidator{},
		APIValidator{},
		PathValidator{},
		PortValidator{},
	)
	return validator.Validate(cfg)
}

// ValidateConfigWith runs a custom validator on a config.
func ValidateConfigWith(cfg *HierarchicalConfig, validators ...Validator) []ConfigError {
	validator := NewCompositeValidator(validators...)
	return validator.Validate(cfg)
}

// HasErrors checks if a set of config errors contains any errors.
func HasErrors(errs []ConfigError) bool {
	for _, e := range errs {
		if e.Severity == string(SeverityError) {
			return true
		}
	}
	return false
}

// FilterErrors returns only errors of the given severity.
func FilterErrors(errs []ConfigError, severity Severity) []ConfigError {
	var filtered []ConfigError
	for _, e := range errs {
		if Severity(e.Severity) == severity {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FormatErrors formats validation errors for display.
func FormatErrors(errs []ConfigError) string {
	if len(errs) == 0 {
		return "configuration is valid"
	}

	var b strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&b, "[%s] %s: %s", strings.ToUpper(e.Severity), e.Path, e.Message)
		if e.Suggestion != "" {
			fmt.Fprintf(&b, " (hint: %s)", e.Suggestion)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
