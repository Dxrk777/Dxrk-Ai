package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

const (
	defaultMaxBodyLogSize = 1024 * 1024
	sensitiveHeaders      = "authorization,proxy-authorization,www-authenticate,cookie,set-cookie,x-api-key,x-auth-token,access-token,refresh-token,secret,password,token,api-key,apikey"
	sensitiveParams       = "password,secret,token,api_key,apikey,access_token,refresh_token,auth_code,code,client_secret"
)

var (
	sensitiveHeaderRegex = regexp.MustCompile(`(?i)^(authorization|proxy-authorization|www-authenticate|cookie|set-cookie|x-api-key|x-auth-token|access-token|refresh-token|secret|password|token|api-key|apikey)$`)
	sensitiveParamRegex  = regexp.MustCompile(`(?i)(password|secret|token|api_key|apikey|access_token|refresh_token|auth_code|code|client_secret)=([^&]+)`)
	creditCardRegex      = regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)
	ssnRegex             = regexp.MustCompile(`\b\d{3}-?\d{2}-?\d{4}\b`)
	emailRegex           = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
)

type LogLevel int

const (
	LogLevelNone LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelNone:
		return "NONE"
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

type LoggingConfig struct {
	Level              LogLevel
	Logger             Logger
	MaxBodySize        int64
	LogRequestHeaders  bool
	LogResponseHeaders bool
	LogRequestBody     bool
	LogResponseBody    bool
	SanitizeHeaders    bool
	SanitizeParams     bool
	SanitizeBody       bool
	IncludeTimestamp   bool
	IncludeDuration    bool
	IncludeRemoteAddr  bool
	IncludeUserAgent   bool
	CustomSanitizers   []SanitizerFunc
}

type SanitizerFunc func(data []byte) []byte

func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Level:              LogLevelInfo,
		Logger:             defaultLogger{},
		MaxBodySize:        defaultMaxBodyLogSize,
		LogRequestHeaders:  true,
		LogResponseHeaders: true,
		LogRequestBody:     true,
		LogResponseBody:    true,
		SanitizeHeaders:    true,
		SanitizeParams:     true,
		SanitizeBody:       true,
		IncludeTimestamp:   true,
		IncludeDuration:    true,
		IncludeRemoteAddr:  true,
		IncludeUserAgent:   true,
	}
}

type defaultLogger struct{}

func (d defaultLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (d defaultLogger) Println(args ...interface{}) {
	fmt.Println(args...)
}

type HTTPLogger struct {
	config *LoggingConfig
	mu     sync.RWMutex
}

func NewHTTPLogger(config *LoggingConfig) *HTTPLogger {
	if config == nil {
		config = DefaultLoggingConfig()
	}
	if config.Logger == nil {
		config.Logger = defaultLogger{}
	}
	if config.MaxBodySize == 0 {
		config.MaxBodySize = defaultMaxBodyLogSize
	}
	return &HTTPLogger{config: config}
}

func (hl *HTTPLogger) LogRequest(req *http.Request) {
	if hl.config.Level < LogLevelInfo {
		return
	}

	hl.mu.RLock()
	config := hl.config
	hl.mu.RUnlock()

	var buf strings.Builder
	hl.writeTimestamp(&buf)
	hl.writeRequestLine(&buf, req)
	hl.writeRemoteAddr(&buf, req)
	hl.writeUserAgent(&buf, req)

	if config.LogRequestHeaders {
		hl.writeHeaders(&buf, "Request Headers", req.Header, config.SanitizeHeaders)
	}

	if config.LogRequestBody && req.Body != nil {
		hl.writeRequestBody(&buf, req, config)
	}

	config.Logger.Printf("%s", buf.String())
}

func (hl *HTTPLogger) LogResponse(resp *http.Response, duration time.Duration) {
	if hl.config.Level < LogLevelInfo {
		return
	}

	hl.mu.RLock()
	config := hl.config
	hl.mu.RUnlock()

	var buf strings.Builder
	hl.writeTimestamp(&buf)
	hl.writeResponseLine(&buf, resp)

	if config.IncludeDuration {
		fmt.Fprintf(&buf, " Duration: %v", duration.Round(time.Millisecond))
	}

	if config.LogResponseHeaders {
		hl.writeHeaders(&buf, "Response Headers", resp.Header, config.SanitizeHeaders)
	}

	if config.LogResponseBody && resp.Body != nil {
		hl.writeResponseBody(&buf, resp, config)
	}

	config.Logger.Printf("%s", buf.String())
}

func (hl *HTTPLogger) LogRoundTrip(req *http.Request, resp *http.Response, duration time.Duration, err error) {
	if hl.config.Level < LogLevelInfo && err == nil {
		return
	}

	hl.mu.RLock()
	config := hl.config
	hl.mu.RUnlock()

	var buf strings.Builder
	hl.writeTimestamp(&buf)

	if err != nil {
		fmt.Fprintf(&buf, " ERROR: %v", err)
		if config.Level >= LogLevelDebug {
			buf.WriteString(" ")
			hl.writeRequestLine(&buf, req)
			hl.writeRemoteAddr(&buf, req)
		}
	} else {
		hl.writeRequestLine(&buf, req)
		hl.writeRemoteAddr(&buf, req)
		hl.writeResponseLine(&buf, resp)
		if config.IncludeDuration {
			fmt.Fprintf(&buf, " Duration: %v", duration.Round(time.Millisecond))
		}
	}

	config.Logger.Printf("%s", buf.String())
}

func (hl *HTTPLogger) writeTimestamp(buf *strings.Builder) {
	if hl.config.IncludeTimestamp {
		buf.WriteString(time.Now().Format("2006-01-02 15:04:05.000 "))
	}
}

func (hl *HTTPLogger) writeRequestLine(buf *strings.Builder, req *http.Request) {
	fmt.Fprintf(buf, " %s %s %s", req.Method, req.URL.RequestURI(), req.Proto)
}

func (hl *HTTPLogger) writeResponseLine(buf *strings.Builder, resp *http.Response) {
	fmt.Fprintf(buf, " %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
}

func (hl *HTTPLogger) writeRemoteAddr(buf *strings.Builder, req *http.Request) {
	if hl.config.IncludeRemoteAddr {
		fmt.Fprintf(buf, " RemoteAddr: %s", getRemoteAddr(req))
	}
}

func (hl *HTTPLogger) writeUserAgent(buf *strings.Builder, req *http.Request) {
	if hl.config.IncludeUserAgent {
		ua := req.UserAgent()
		if ua != "" {
			fmt.Fprintf(buf, " User-Agent: %s", ua)
		}
	}
}

func (hl *HTTPLogger) writeHeaders(buf *strings.Builder, title string, headers http.Header, sanitize bool) {
	fmt.Fprintf(buf, "\n%s:", title)
	for key, values := range headers {
		if sanitize && sensitiveHeaderRegex.MatchString(key) {
			fmt.Fprintf(buf, "\n  %s: [REDACTED]", key)
			continue
		}
		for _, v := range values {
			fmt.Fprintf(buf, "\n  %s: %s", key, v)
		}
	}
}

func (hl *HTTPLogger) writeRequestBody(buf *strings.Builder, req *http.Request, config *LoggingConfig) {
	body, err := hl.readBody(req.Body, config.MaxBodySize)
	if err != nil {
		fmt.Fprintf(buf, "\nRequest Body: [error reading: %v]", err)
		return
	}

	if len(body) == 0 {
		buf.WriteString("\nRequest Body: [empty]")
		return
	}

	if config.SanitizeBody {
		body = hl.sanitizeBody(body)
	}

	if config.SanitizeParams {
		body = hl.sanitizeParams(body)
	}

	for _, sanitizer := range config.CustomSanitizers {
		body = sanitizer(body)
	}

	fmt.Fprintf(buf, "\nRequest Body (%d bytes): %s", len(body), string(body))
}

func (hl *HTTPLogger) writeResponseBody(buf *strings.Builder, resp *http.Response, config *LoggingConfig) {
	body, err := hl.readBody(resp.Body, config.MaxBodySize)
	if err != nil {
		fmt.Fprintf(buf, "\nResponse Body: [error reading: %v]", err)
		return
	}

	if len(body) == 0 {
		buf.WriteString("\nResponse Body: [empty]")
		return
	}

	if config.SanitizeBody {
		body = hl.sanitizeBody(body)
	}

	for _, sanitizer := range config.CustomSanitizers {
		body = sanitizer(body)
	}

	fmt.Fprintf(buf, "\nResponse Body (%d bytes): %s", len(body), string(body))
}

func (hl *HTTPLogger) readBody(body io.ReadCloser, maxSize int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	limited := io.LimitReader(body, maxSize)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	if len(data) == int(maxSize) {
		buf := make([]byte, 1)
		n, _ := body.Read(buf)
		if n > 0 {
			data = append(data, []byte("... [truncated]")...)
		}
	}

	return data, nil
}

func (hl *HTTPLogger) sanitizeBody(data []byte) []byte {
	result := data

	result = creditCardRegex.ReplaceAll(result, []byte("[CREDIT_CARD_REDACTED]"))
	result = ssnRegex.ReplaceAll(result, []byte("[SSN_REDACTED]"))
	result = emailRegex.ReplaceAll(result, []byte("[EMAIL_REDACTED]"))

	return result
}

func (hl *HTTPLogger) sanitizeParams(data []byte) []byte {
	return sensitiveParamRegex.ReplaceAll(data, []byte("$1=[REDACTED]"))
}

func (hl *HTTPLogger) SetLevel(level LogLevel) {
	hl.mu.Lock()
	defer hl.mu.Unlock()
	hl.config.Level = level
}

func (hl *HTTPLogger) GetLevel() LogLevel {
	hl.mu.RLock()
	defer hl.mu.RUnlock()
	return hl.config.Level
}

func (hl *HTTPLogger) SetConfig(config *LoggingConfig) {
	hl.mu.Lock()
	defer hl.mu.Unlock()
	hl.config = config
}

func (hl *HTTPLogger) GetConfig() *LoggingConfig {
	hl.mu.RLock()
	defer hl.mu.RUnlock()
	return hl.config
}

func (hl *HTTPLogger) Middleware(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		start := time.Now()
		hl.LogRequest(req)

		resp, err := next.RoundTrip(req)
		duration := time.Since(start)

		if err != nil {
			hl.LogRoundTrip(req, nil, duration, err)
			return nil, err
		}

		hl.LogResponse(resp, duration)
		hl.LogRoundTrip(req, resp, duration, nil)

		return resp, nil
	})
}

type LoggedTransport struct {
	transport http.RoundTripper
	logger    *HTTPLogger
}

func NewLoggedTransport(transport http.RoundTripper, logger *HTTPLogger) *LoggedTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &LoggedTransport{
		transport: transport,
		logger:    logger,
	}
}

func (lt *LoggedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	lt.logger.LogRequest(req)

	resp, err := lt.transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		lt.logger.LogRoundTrip(req, nil, duration, err)
		return nil, err
	}

	lt.logger.LogResponse(resp, duration)
	lt.logger.LogRoundTrip(req, resp, duration, nil)

	return resp, nil
}

func getRemoteAddr(req *http.Request) string {
	if req == nil {
		return strconst.StrUnknown
	}

	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return req.RemoteAddr
}

func DumpRequest(req *http.Request, body bool) ([]byte, error) {
	return httputil.DumpRequest(req, body)
}

func DumpRequestOut(req *http.Request, body bool) ([]byte, error) {
	return httputil.DumpRequestOut(req, body)
}

func DumpResponse(resp *http.Response, body bool) ([]byte, error) {
	return httputil.DumpResponse(resp, body)
}

type contextKey string

const loggerContextKey contextKey = "http_logger"

func WithLogger(ctx context.Context, logger *HTTPLogger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

func LoggerFromContext(ctx context.Context) *HTTPLogger {
	if logger, ok := ctx.Value(loggerContextKey).(*HTTPLogger); ok {
		return logger
	}
	return nil
}

func SanitizeHeaders(headers http.Header) http.Header {
	sanitized := make(http.Header)
	for key, values := range headers {
		if sensitiveHeaderRegex.MatchString(key) {
			sanitized[key] = []string{"[REDACTED]"}
		} else {
			sanitized[key] = values
		}
	}
	return sanitized
}

func SanitizeURL(url string) string {
	return sensitiveParamRegex.ReplaceAllString(url, "$1=[REDACTED]")
}

func SanitizeBody(body []byte) []byte {
	result := body
	result = creditCardRegex.ReplaceAll(result, []byte("[CREDIT_CARD_REDACTED]"))
	result = ssnRegex.ReplaceAll(result, []byte("[SSN_REDACTED]"))
	result = emailRegex.ReplaceAll(result, []byte("[EMAIL_REDACTED]"))
	result = sensitiveParamRegex.ReplaceAll(result, []byte("$1=[REDACTED]"))
	return result
}
