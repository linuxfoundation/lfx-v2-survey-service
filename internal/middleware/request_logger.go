// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
)

// requestFailureKey is the context key under which RequestLoggerMiddleware keeps a
// per-request holder for the error that made the request fail.
type requestFailureKey struct{}

// requestFailure holds the cause of a failed request until the response status is known.
type requestFailure struct {
	mu  sync.Mutex
	err error
}

func (f *requestFailure) set(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *requestFailure) get() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// RecordRequestFailure notes why the current request failed, so RequestLoggerMiddleware
// can log it once after the response is written, using the status code that was actually
// sent. The status a handler intends to send and the one the transport sends can differ
// (for example when the generated encoder does not declare an error type), so the log
// follows the transport rather than the handler.
//
// It reports whether the request is being tracked. That is false outside
// RequestLoggerMiddleware, for example when a handler is called directly in a test, and the
// caller should then log the failure itself with LogRequestFailure.
func RecordRequestFailure(ctx context.Context, err error) bool {
	f, ok := ctx.Value(requestFailureKey{}).(*requestFailure)
	if !ok {
		return false
	}
	f.set(err)
	return true
}

// LogRequestFailure records why a request failed. Server-side failures (5xx) are logged at
// Error. Client-side failures (4xx) are expected outcomes and stay at Debug, so they do not
// show up as errors in dashboards or alerting.
func LogRequestFailure(ctx context.Context, status int, err error) {
	level := slog.LevelDebug
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	slog.Log(ctx, level, "request failed", "status", status, "error", err)
}

// RequestLoggerMiddleware creates a middleware that logs HTTP requests
func RequestLoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Add request URL attributes to the context so that they can be used in all request handler logs
			ctx := r.Context()
			ctx = logging.AppendCtx(ctx, slog.String("method", r.Method))
			ctx = logging.AppendCtx(ctx, slog.String("path", r.URL.Path))
			ctx = logging.AppendCtx(ctx, slog.String("query", r.URL.RawQuery))
			ctx = logging.AppendCtx(ctx, slog.String("host", r.Host))
			ctx = logging.AppendCtx(ctx, slog.String("user_agent", r.UserAgent()))
			ctx = logging.AppendCtx(ctx, slog.String("remote_addr", r.RemoteAddr))

			if r.Header.Get(constants.EtagHeader) != "" {
				ctx = logging.AppendCtx(ctx, slog.String("req_header_etag", r.Header.Get(constants.EtagHeader)))
			}

			isHealthCheck := constants.IsHealthPath(r.URL.Path)

			// Handlers record why a request failed here; it is logged below, once the
			// response status is known.
			failure := &requestFailure{}
			ctx = context.WithValue(ctx, requestFailureKey{}, failure)

			// Create a new request with the updated context
			r = r.WithContext(ctx)

			// Create a response writer wrapper to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			if isHealthCheck {
				slog.DebugContext(ctx, "HTTP request")
			} else {
				slog.InfoContext(ctx, "HTTP request")
			}

			// Call the next handler
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start)

			if err := failure.get(); err != nil {
				LogRequestFailure(ctx, ww.statusCode, err)
			}

			if isHealthCheck {
				slog.DebugContext(ctx, "HTTP response", "status", ww.statusCode, "duration", duration.String())
				return
			}
			// Log the response
			slog.InfoContext(ctx, "HTTP response", "status", ww.statusCode, "duration", duration.String())
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
