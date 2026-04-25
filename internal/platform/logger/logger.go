package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type traceIDKey struct{}

var (
	defaultLogger *zap.Logger
	atomicLevel   zap.AtomicLevel
)

// Init initializes the global logger.
// isDev=true → zap.NewDevelopment (colored console, DEBUG level).
// isDev=false → zap.NewProduction (JSON stdout, INFO level).
func Init(level string, isDev bool) {
	atomicLevel = zap.NewAtomicLevel()

	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}
	atomicLevel.SetLevel(lvl)

	var err error
	if isDev {
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = atomicLevel
		defaultLogger, err = cfg.Build()
	} else {
		cfg := zap.NewProductionConfig()
		cfg.Level = atomicLevel
		cfg.OutputPaths = []string{"stdout"}
		defaultLogger, err = cfg.Build()
	}
	if err != nil {
		// Fallback to a no-op logger on unexpected build failure.
		defaultLogger = zap.NewNop()
	}
}

// L returns the global zap.Logger.
func L() *zap.Logger {
	if defaultLogger == nil {
		defaultLogger = zap.NewNop()
	}
	return defaultLogger
}

// SetLevel adjusts the log level at runtime.
func SetLevel(level zapcore.Level) {
	atomicLevel.SetLevel(level)
}

// WithTraceID stores a trace ID in ctx.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// GetTraceID retrieves the trace ID from ctx.
// Returns empty string when ctx is nil.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(traceIDKey{}).(string); ok {
		return id
	}
	return ""
}

// FromContext returns a child logger with trace_id field when present.
func FromContext(ctx context.Context) *zap.Logger {
	if id := GetTraceID(ctx); id != "" {
		return L().With(zap.String("trace_id", id))
	}
	return L()
}

// maskSecret masks a secret string for logging.
func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****"
}

// MaskSecret exposes maskSecret for use in other packages.
func MaskSecret(s string) string {
	return maskSecret(s)
}

// ---------- backward-compat shims (used by grpc/server.go and other packages) ----------

// LogOption is kept for backward compatibility with existing call sites.
type LogOption func(*legacyEntry)

type legacyEntry struct {
	userID    string
	roomID    string
	messageID string
	action    string
	details   map[string]interface{}
}

// WithUserID attaches a user_id field.
func WithUserID(userID string) LogOption {
	return func(e *legacyEntry) { e.userID = userID }
}

// WithRoomID attaches a room_id field.
func WithRoomID(roomID string) LogOption {
	return func(e *legacyEntry) { e.roomID = roomID }
}

// WithMessageID attaches a message_id field.
func WithMessageID(messageID string) LogOption {
	return func(e *legacyEntry) { e.messageID = messageID }
}

// WithAction attaches an action field.
func WithAction(action string) LogOption {
	return func(e *legacyEntry) { e.action = action }
}

// WithDetails attaches arbitrary detail fields.
func WithDetails(details map[string]interface{}) LogOption {
	return func(e *legacyEntry) { e.details = details }
}

func buildFields(opts []LogOption) []zap.Field {
	e := &legacyEntry{}
	for _, o := range opts {
		o(e)
	}
	var fields []zap.Field
	if e.userID != "" {
		fields = append(fields, zap.String("user_id", e.userID))
	}
	if e.roomID != "" {
		fields = append(fields, zap.String("room_id", e.roomID))
	}
	if e.messageID != "" {
		fields = append(fields, zap.String("message_id", e.messageID))
	}
	if e.action != "" {
		fields = append(fields, zap.String("action", e.action))
	}
	for k, v := range e.details {
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

// Info logs at INFO level with optional legacy options.
func Info(ctx context.Context, msg string, opts ...LogOption) {
	FromContext(ctx).Info(msg, buildFields(opts)...)
}

// Warning logs at WARN level with optional legacy options.
func Warning(ctx context.Context, msg string, opts ...LogOption) {
	FromContext(ctx).Warn(msg, buildFields(opts)...)
}

// Error logs at ERROR level with optional legacy options.
func Error(ctx context.Context, msg string, opts ...LogOption) {
	FromContext(ctx).Error(msg, buildFields(opts)...)
}

// Debug logs at DEBUG level with optional legacy options.
func Debug(ctx context.Context, msg string, opts ...LogOption) {
	FromContext(ctx).Debug(msg, buildFields(opts)...)
}

// Infof logs a formatted string at INFO level.
func Infof(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Infof(format, args...)
}

// Warningf logs a formatted string at WARN level.
func Warningf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Warnf(format, args...)
}

// Errorf logs a formatted string at ERROR level.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Errorf(format, args...)
}

// LogInfof logs at INFO without a context (legacy call site shim).
func LogInfof(format string, v ...interface{}) {
	L().Sugar().Infof(format, v...)
}

// LogWarnf logs at WARN without a context (legacy call site shim).
func LogWarnf(format string, v ...interface{}) {
	L().Sugar().Warnf(format, v...)
}

// LogErrorf logs at ERROR without a context (legacy call site shim).
func LogErrorf(format string, v ...interface{}) {
	L().Sugar().Errorf(format, v...)
}
