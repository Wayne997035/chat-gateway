package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"chat-gateway/internal/platform/config"

	"github.com/google/uuid"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

// Severity GCP Cloud Logging 嚴重級別
type Severity string

const (
	SeverityDefault   Severity = "DEFAULT"
	SeverityDebug     Severity = "DEBUG"
	SeverityInfo      Severity = "INFO"
	SeverityNotice    Severity = "NOTICE"
	SeverityWarning   Severity = "WARNING"
	SeverityError     Severity = "ERROR"
	SeverityCritical  Severity = "CRITICAL"
	SeverityAlert     Severity = "ALERT"
	SeverityEmergency Severity = "EMERGENCY"
)

// LogEntry GCP Cloud Logging 格式的日誌條目
type LogEntry struct {
	Severity       Severity          `json:"severity"`
	Message        string            `json:"message"`
	Timestamp      string            `json:"timestamp"`       // RFC3339 格式
	TraceID        string            `json:"trace,omitempty"` // GCP trace ID 格式: projects/[PROJECT_ID]/traces/[TRACE_ID]
	SpanID         string            `json:"spanId,omitempty"`
	HTTPRequest    *HTTPRequest      `json:"httpRequest,omitempty"`
	SourceLocation *SourceLocation   `json:"sourceLocation,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Operation      *Operation        `json:"operation,omitempty"`
	InsertID       string            `json:"insertId,omitempty"` // 用於去重
	// 自定義欄位
	UserID    string                 `json:"userId,omitempty"`
	RoomID    string                 `json:"roomId,omitempty"`
	MessageID string                 `json:"messageId,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HTTPRequest HTTP 請求信息
type HTTPRequest struct {
	RequestMethod string `json:"requestMethod,omitempty"`
	RequestURL    string `json:"requestUrl,omitempty"`
	RequestSize   int64  `json:"requestSize,omitempty"`
	Status        int    `json:"status,omitempty"`
	ResponseSize  int64  `json:"responseSize,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
	RemoteIP      string `json:"remoteIp,omitempty"`
	ServerIP      string `json:"serverIp,omitempty"`
	Referer       string `json:"referer,omitempty"`
	Latency       string `json:"latency,omitempty"` // 格式: "1.234s"
	Protocol      string `json:"protocol,omitempty"`
}

// SourceLocation 源代碼位置
type SourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     int64  `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// Operation 操作信息（用於追蹤長時間運行的操作）
type Operation struct {
	ID       string `json:"id,omitempty"`
	Producer string `json:"producer,omitempty"`
	First    bool   `json:"first,omitempty"`
	Last     bool   `json:"last,omitempty"`
}

var (
	logWriter   io.Writer
	projectID   string
	serviceName string
)

// InitLogger 初始化 GCP 格式日誌系統
func InitLogger() error {
	// 從環境變數取得配置
	logDir := os.Getenv("LOG_PATH")
	if logDir == "" {
		logDir = "./logs"
	}

	projectID = os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		projectID = "local-dev"
	}

	serviceName = os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "chat-gateway"
	}

	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return err
	}

	// 從配置檔案讀取日誌輪轉設定
	cfg := config.Get()
	rotationTime := 24
	maxAge := 30
	maxSize := 100

	if cfg != nil && cfg.Log.RotationTimeHours > 0 {
		rotationTime = cfg.Log.RotationTimeHours
	}
	if cfg != nil && cfg.Log.MaxAgeDays > 0 {
		maxAge = cfg.Log.MaxAgeDays
	}
	if cfg != nil && cfg.Log.MaxSizeMB > 0 {
		maxSize = cfg.Log.MaxSizeMB
	}

	// 設定日誌輪轉
	logFileName := filepath.Join(logDir, "app.log")
	writer, err := rotatelogs.New(
		logFileName+".%Y%m%d",
		rotatelogs.WithLinkName(logFileName),
		rotatelogs.WithRotationTime(time.Duration(rotationTime)*time.Hour),
		rotatelogs.WithMaxAge(time.Duration(maxAge)*24*time.Hour),
		rotatelogs.WithRotationSize(int64(maxSize)*1024*1024),
	)
	if err != nil {
		return err
	}

	logWriter = writer

	return nil
}

// CloseLogger 關閉日誌檔案
func CloseLogger() {
	if logWriter != nil {
		if closer, ok := logWriter.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
			}
		}
	}
}

// writeLog 寫入日誌（內部方法）
func writeLog(entry *LogEntry) {
	// 生成 JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	// 寫入檔案和控制台
	if logWriter != nil {
		logWriter.Write(append(jsonData, '\n'))
	}
	os.Stdout.Write(append(jsonData, '\n'))
}

// getSourceLocation 獲取源代碼位置
func getSourceLocation(skip int) *SourceLocation {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	funcName := "unknown"
	if fn != nil {
		funcName = fn.Name()
	}

	return &SourceLocation{
		File:     filepath.Base(file),
		Line:     int64(line),
		Function: funcName,
	}
}

// generateInsertID 生成去重 ID
func generateInsertID() string {
	return uuid.New().String()
}

// GetTraceID 從 context 獲取 trace ID
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return formatTraceID(traceID)
	}
	return ""
}

// formatTraceID 格式化 trace ID 為 GCP 格式
func formatTraceID(traceID string) string {
	if traceID == "" {
		return ""
	}
	return fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)
}

// NewTraceID 生成新的 trace ID
func NewTraceID() string {
	return uuid.New().String()
}

// WithTraceID 將 trace ID 添加到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, "trace_id", traceID)
}

// Log 通用日誌方法
func Log(ctx context.Context, severity Severity, message string, opts ...LogOption) {
	entry := &LogEntry{
		Severity:       severity,
		Message:        message,
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		TraceID:        GetTraceID(ctx),
		SourceLocation: getSourceLocation(3),
		InsertID:       generateInsertID(),
		Labels: map[string]string{
			"service": serviceName,
		},
	}

	// 應用選項
	for _, opt := range opts {
		opt(entry)
	}

	writeLog(entry)
}

// LogOption 日誌選項
type LogOption func(*LogEntry)

// WithUserID 添加用戶 ID
func WithUserID(userID string) LogOption {
	return func(e *LogEntry) {
		e.UserID = userID
	}
}

// WithRoomID 添加聊天室 ID
func WithRoomID(roomID string) LogOption {
	return func(e *LogEntry) {
		e.RoomID = roomID
	}
}

// WithMessageID 添加消息 ID
func WithMessageID(messageID string) LogOption {
	return func(e *LogEntry) {
		e.MessageID = messageID
	}
}

// WithAction 添加操作
func WithAction(action string) LogOption {
	return func(e *LogEntry) {
		e.Action = action
	}
}

// WithDetails 添加詳細信息
func WithDetails(details map[string]interface{}) LogOption {
	return func(e *LogEntry) {
		e.Details = details
	}
}

// WithHTTPRequest 添加 HTTP 請求信息
func WithHTTPRequest(req *HTTPRequest) LogOption {
	return func(e *LogEntry) {
		e.HTTPRequest = req
	}
}

// WithLabels 添加標籤
func WithLabels(labels map[string]string) LogOption {
	return func(e *LogEntry) {
		if e.Labels == nil {
			e.Labels = make(map[string]string)
		}
		for k, v := range labels {
			e.Labels[k] = v
		}
	}
}

// 便捷方法

// Debug 記錄 DEBUG 級別日誌
func Debug(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityDebug, message, opts...)
}

// Info 記錄 INFO 級別日誌
func Info(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityInfo, message, opts...)
}

// Notice 記錄 NOTICE 級別日誌
func Notice(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityNotice, message, opts...)
}

// Warning 記錄 WARNING 級別日誌
func Warning(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityWarning, message, opts...)
}

// Error 記錄 ERROR 級別日誌
func Error(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityError, message, opts...)
}

// Critical 記錄 CRITICAL 級別日誌
func Critical(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityCritical, message, opts...)
}

// Alert 記錄 ALERT 級別日誌
func Alert(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityAlert, message, opts...)
}

// Emergency 記錄 EMERGENCY 級別日誌
func Emergency(ctx context.Context, message string, opts ...LogOption) {
	Log(ctx, SeverityEmergency, message, opts...)
}

// Infof 格式化 INFO 日誌
func Infof(ctx context.Context, format string, args ...interface{}) {
	Info(ctx, fmt.Sprintf(format, args...))
}

// Warningf 格式化 WARNING 日誌
func Warningf(ctx context.Context, format string, args ...interface{}) {
	Warning(ctx, fmt.Sprintf(format, args...))
}

// Errorf 格式化 ERROR 日誌
func Errorf(ctx context.Context, format string, args ...interface{}) {
	Error(ctx, fmt.Sprintf(format, args...))
}

// 向後兼容的舊方法（會逐步廢棄）

// LogInfof 舊版 INFO 日誌（向後兼容）
func LogInfof(format string, v ...interface{}) {
	Info(context.Background(), fmt.Sprintf(format, v...))
}

// LogWarnf 舊版 WARNING 日誌（向後兼容）
func LogWarnf(format string, v ...interface{}) {
	Warning(context.Background(), fmt.Sprintf(format, v...))
}

// LogErrorf 舊版 ERROR 日誌（向後兼容）
func LogErrorf(format string, v ...interface{}) {
	Error(context.Background(), fmt.Sprintf(format, v...))
}
