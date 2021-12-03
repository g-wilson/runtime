package ctxlog

import (
	"context"

	"github.com/sirupsen/logrus"
)

const (
	// ServiceKey is the log field for the service name
	ServiceKey = "svc"
	// LevelKey is the log field for the log level
	LevelKey = "lvl"
	// MessageKey is the log field for the log message
	MessageKey = "msg"
	// TimestampKey is the log field for the log timestamp
	TimestampKey = "t"
)

type ctxLogKey string

var logKey = ctxLogKey("ctxlog")

// Create creates a new Logrus Entry with defaults
func Create(servicename, format, level string) *logrus.Entry {
	logger := logrus.WithField("svc", servicename)

	switch format {
	case "json":
		logger.Logger.Formatter = &logrus.JSONFormatter{
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyLevel: LevelKey,
				logrus.FieldKeyMsg:   MessageKey,
				logrus.FieldKeyTime:  TimestampKey,
			},
		}

	default:
		logger.Logger.Formatter = &logrus.TextFormatter{}
	}

	levelEnum, _ := logrus.ParseLevel(level)
	logger.Logger.Level = levelEnum

	return logger
}

// ContextSafeLogger is an abstraction which allows the context to remain lightweight and hold just a pointer to a logger
type ContextSafeLogger struct {
	entry *logrus.Entry
}

// SetContext adds a logger to a context
func SetContext(ctx context.Context, log *logrus.Entry) context.Context {
	return context.WithValue(ctx, logKey, &ContextSafeLogger{entry: log})
}

// FromContext retrieves a logger from the context
func FromContext(ctx context.Context) *ContextSafeLogger {
	if ctxLogger, ok := ctx.Value(logKey).(*ContextSafeLogger); ok {
		return ctxLogger
	}

	return nil
}

// Update replaces the logger
func (l *ContextSafeLogger) Update(en *logrus.Entry) {
	l.entry = en
}

// Entry returns the logger
func (l *ContextSafeLogger) Entry() *logrus.Entry {
	return l.entry
}
