package health

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/driver"
	"chat-gateway/internal/platform/logger"

	"github.com/gin-gonic/gin"
)

const (
	// 健康狀態常數.
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"
	statusWarning   = "warning"

	// 記憶體相關常數.
	memoryMB        = 1024 * 1024
	memoryThreshold = 1024 // 1GB

	// 超時常數.
	dbTimeout = 5 * time.Second
)

// Handler 健康檢查處理器.
type Handler struct{}

// NewHealthHandler 創建新的健康檢查處理器.
func NewHealthHandler() *Handler {
	return &Handler{}
}

// HealthCheck 健康檢查端點.
func (h *Handler) HealthCheck(c *gin.Context) {
	cfg := config.Get()

	// 檢查資料庫連線.
	dbStatus := statusHealthy
	dbError := ""
	dbDetails := gin.H{}

	if err := h.checkDatabase(); err != nil {
		dbStatus = statusUnhealthy
		dbError = err.Error()
		logger.LogErrorf("健康檢查 - 資料庫連線失敗: %v", err)
	} else {
		// 添加資料庫詳細資訊.
		dbDetails = gin.H{
			"connected": driver.IsConnected(),
			"database":  cfg.Database.Mongo.Database,
		}
	}

	// 檢查系統資源.
	systemStatus := h.checkSystemResources()

	// 從環境變數讀取版本，沒有則用預設值
	appVersion := os.Getenv("APP_VERSION")
	if appVersion == "" {
		appVersion = "NO_VERSION_SET" // 預設版本號
	}

	// 回應格式.
	response := gin.H{
		"status":    statusHealthy,
		"timestamp": time.Now().Unix(),
		"app": gin.H{
			"name":    cfg.App.Name,
			"version": appVersion,
			"debug":   cfg.App.Debug,
		},
		"database": gin.H{
			"status":  dbStatus,
			"error":   dbError,
			"details": dbDetails,
		},
		"system": gin.H{
			"status":  systemStatus.Status,
			"details": systemStatus.Details,
			"uptime":  time.Since(startTime).String(),
		},
	}

	// 如果資料庫不健康，將整體狀態設為 degraded.
	if dbStatus == statusUnhealthy {
		response["status"] = "degraded"
	}

	// 即使資料庫不健康，也回傳 200 狀態碼，讓監控系統知道服務本身是正常的.
	// 資料庫狀態會在回應中顯示.
	c.JSON(http.StatusOK, response)
}

// SystemStatus 系統狀態.
type SystemStatus struct {
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details"`
}

// checkSystemResources 檢查系統資源.
func (h *Handler) checkSystemResources() SystemStatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	details := map[string]interface{}{
		"goroutines": runtime.NumGoroutine(),
		"memory": gin.H{
			"alloc":       fmt.Sprintf("%.2f MB", float64(m.Alloc)/memoryMB),
			"total_alloc": fmt.Sprintf("%.2f MB", float64(m.TotalAlloc)/memoryMB),
			"sys":         fmt.Sprintf("%.2f MB", float64(m.Sys)/memoryMB),
			"num_gc":      m.NumGC,
		},
		"cpu": gin.H{
			"num_cpu": runtime.NumCPU(),
		},
	}

	// 檢查記憶體使用是否過高（超過 1GB 視為警告）
	memoryUsage := m.Sys / memoryMB // MB
	status := statusHealthy
	if memoryUsage > memoryThreshold {
		status = statusWarning
		details["memory_warning"] = "Memory usage is high"
	}

	return SystemStatus{
		Status:  status,
		Details: details,
	}
}

// checkDatabase 檢查資料庫連線.
func (h *Handler) checkDatabase() error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// 嘗試 ping 資料庫
	db := driver.GetMongoDatabase()
	if db == nil {
		return fmt.Errorf("database connection not available")
	}

	// 執行簡單的 ping 操作.
	return db.Client().Ping(ctx, nil)
}

// 記錄服務啟動時間.
var startTime = time.Now()
