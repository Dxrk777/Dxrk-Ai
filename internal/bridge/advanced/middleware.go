package advanced

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type Middleware interface {
	Wrap(http.Handler) http.Handler
}

type RateLimiter struct {
	mu       sync.Mutex
	rate     float64
	burst    int
	tokens   float64
	lastTime time.Time
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:     rps,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.tokens += elapsed * rl.rate
	if rl.tokens > float64(rl.burst) {
		rl.tokens = float64(rl.burst)
	}
	rl.lastTime = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	tokens := rl.tokens + elapsed*rl.rate
	if tokens > float64(rl.burst) {
		tokens = float64(rl.burst)
	}
	return tokens
}

type RateLimitMiddleware struct {
	limiter *RateLimiter
	next    http.Handler
}

func NewRateLimitMiddleware(rps float64, burst int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: NewRateLimiter(rps, burst),
	}
}

func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type RetryMiddleware struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	next        http.Handler
}

func NewRetryMiddleware(maxAttempts int, baseDelay, maxDelay time.Duration) *RetryMiddleware {
	return &RetryMiddleware{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
	}
}

func (m *RetryMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var lastErr error
		for attempt := 0; attempt < m.maxAttempts; attempt++ {
			if attempt > 0 {
				delay := m.baseDelay * time.Duration(1<<(attempt-1))
				if delay > m.maxDelay {
					delay = m.maxDelay
				}
				time.Sleep(delay)
			}

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			if rw.statusCode < 500 {
				return
			}
			lastErr = &retryError{status: rw.statusCode, attempt: attempt + 1}
		}
		if lastErr != nil {
			http.Error(w, lastErr.Error(), http.StatusServiceUnavailable)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

type retryError struct {
	status  int
	attempt int
}

func (e *retryError) Error() string {
	return "request failed after retries"
}

type LoggingMiddleware struct {
	next http.Handler
}

func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

func (m *LoggingMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s %d %v",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			rw.statusCode,
			duration,
		)
	})
}

type AuthMiddleware struct {
	validTokens map[string]bool
	next        http.Handler
}

func NewAuthMiddleware(tokens ...string) *AuthMiddleware {
	valid := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		valid[t] = true
	}
	return &AuthMiddleware{validTokens: valid}
}

func (m *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !m.validTokens[token] {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type HeadersMiddleware struct {
	headers map[string]string
	next    http.Handler
}

func NewHeadersMiddleware(headers map[string]string) *HeadersMiddleware {
	return &HeadersMiddleware{headers: headers}
}

func (m *HeadersMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range m.headers {
			w.Header().Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}

type RecoveryMiddleware struct {
	next http.Handler
}

func NewRecoveryMiddleware() *RecoveryMiddleware {
	return &RecoveryMiddleware{}
}

func (m *RecoveryMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type TimeoutMiddleware struct {
	timeout time.Duration
	next    http.Handler
}

func NewTimeoutMiddleware(timeout time.Duration) *TimeoutMiddleware {
	return &TimeoutMiddleware{timeout: timeout}
}

func (m *TimeoutMiddleware) Wrap(next http.Handler) http.Handler {
	m.next = next
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := make(chan struct{})
		go func() {
			next.ServeHTTP(w, r)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(m.timeout):
			http.Error(w, "request timeout", http.StatusGatewayTimeout)
		}
	})
}

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i].Wrap(h)
	}
	return h
}
