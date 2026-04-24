package logger

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestInit_DevMode(t *testing.T) {
	Init("debug", true)
	l := L()
	if l == nil {
		t.Fatal("expected non-nil logger after Init")
	}
}

func TestInit_ProdMode(t *testing.T) {
	Init("info", false)
	l := L()
	if l == nil {
		t.Fatal("expected non-nil logger after Init")
	}
}

func TestInit_InvalidLevel_FallsBackToInfo(t *testing.T) {
	Init("notALevel", false)
	l := L()
	if l == nil {
		t.Fatal("expected non-nil logger even with invalid level")
	}
}

func TestWithTraceIDAndGetTraceID(t *testing.T) {
	cases := []struct {
		name    string
		traceID string
		wantID  string
	}{
		{name: "happy path", traceID: "abc123", wantID: "abc123"},
		{name: "empty trace id", traceID: "", wantID: ""},
		{name: "long trace id", traceID: "deadbeefdeadbeefdeadbeefdeadbeef", wantID: "deadbeefdeadbeefdeadbeefdeadbeef"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.traceID != "" {
				ctx = WithTraceID(ctx, tc.traceID)
			}
			got := GetTraceID(ctx)
			if got != tc.wantID {
				t.Errorf("GetTraceID() = %q, want %q", got, tc.wantID)
			}
		})
	}
}

func TestGetTraceID_NoTraceID(t *testing.T) {
	got := GetTraceID(context.Background())
	if got != "" {
		t.Errorf("expected empty string for context without trace ID, got %q", got)
	}
}

func TestFromContext_WithTraceID(t *testing.T) {
	Init("debug", true)
	ctx := WithTraceID(context.Background(), "trace-xyz")
	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext must never return nil")
	}
}

func TestFromContext_WithoutTraceID(t *testing.T) {
	Init("debug", true)
	l := FromContext(context.Background())
	if l == nil {
		t.Fatal("FromContext must never return nil when no trace ID present")
	}
}

func TestSetLevel(t *testing.T) {
	Init("info", false)

	cases := []struct {
		name  string
		level zapcore.Level
	}{
		{"set debug", zapcore.DebugLevel},
		{"set warn", zapcore.WarnLevel},
		{"set error", zapcore.ErrorLevel},
		{"set info", zapcore.InfoLevel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SetLevel(tc.level)
			if atomicLevel.Level() != tc.level {
				t.Errorf("SetLevel(%v): got %v", tc.level, atomicLevel.Level())
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"short secret", "abc", "****"},
		{"exactly 8 chars", "12345678", "****"},
		{"9 chars", "123456789", "1234****"},
		{"long secret", "supersecrettoken", "supe****"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskSecret(tc.input)
			if got != tc.want {
				t.Errorf("MaskSecret(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLegacyLogOptions(t *testing.T) {
	Init("debug", true)
	ctx := context.Background()

	// Ensure all legacy logger functions do not panic.
	Info(ctx, "test info", WithUserID("u1"), WithRoomID("r1"), WithMessageID("m1"), WithAction("test"), WithDetails(map[string]interface{}{"k": "v"}))
	Warning(ctx, "test warning")
	Error(ctx, "test error")
	Debug(ctx, "test debug")
	Infof(ctx, "infof %s", "hello")
	Warningf(ctx, "warnf %s", "hello")
	Errorf(ctx, "errorf %s", "hello")
	LogInfof("loginf %s", "hello")
	LogWarnf("logwarnf %s", "hello")
	LogErrorf("logerrorf %s", "hello")
}

func TestBuildFields_AllOptions(t *testing.T) {
	opts := []LogOption{
		WithUserID("u123"),
		WithRoomID("r456"),
		WithMessageID("m789"),
		WithAction("send"),
		WithDetails(map[string]interface{}{"status": "ok", "count": 3}),
	}
	fields := buildFields(opts)

	found := make(map[string]bool)
	for _, f := range fields {
		found[f.Key] = true
	}

	for _, key := range []string{"user_id", "room_id", "message_id", "action", "status", "count"} {
		if !found[key] {
			t.Errorf("expected field %q in built fields", key)
		}
	}
}

func TestL_NilInitialization(t *testing.T) {
	// Temporarily nil out defaultLogger to test fallback.
	orig := defaultLogger
	defaultLogger = nil
	t.Cleanup(func() { defaultLogger = orig })

	l := L()
	if l == nil {
		t.Fatal("L() must never return nil")
	}
	// Nop logger should be safe to use.
	l.Info("should not panic", zap.String("key", "val"))
}
