// Package logger provides a custom logging solution built on top of Uber's Zap logging library.
// It includes functionality for creating and configuring a logger instance and HTTP middleware
// to log incoming HTTP requests.
package logger

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
)

// Logger wraps the zap.Logger to provide additional logging functionality.
type Logger struct {
	*zap.Logger
}

// newLogger initializes a new Logger instance using the production configuration of Zap.
// In case of an error during creation, it logs the error using the standard log package.
func newLogger() *Logger {
	customLog, err := zap.NewProduction()
	if err != nil {
		log.Println(err)
	}
	return &Logger{Logger: customLog}
}

// CreateLogger creates and configures a Logger with the specified log level.
// It parses the provided level, applies it to the production configuration, and builds a new Zap logger.
func CreateLogger(level string) (customLog *Logger, err error) {
	log := newLogger()
	defer log.Sync()

	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return log, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return log, err
	}

	log.Logger = zl
	return log, nil
}

// WithLogging returns HTTP middleware that logs incoming HTTP requests.
// It wraps the provided HTTP handler, recording details such as method, URI, status code,
// duration, and response size using the Zap logger.
func (log *Logger) WithLogging() func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				log.Info("served",
					zap.String("method", r.Method),
					zap.String("uri", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Duration("duration", time.Since(t1)),
					zap.Int("size", ww.BytesWritten()))
			}()
			h.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
