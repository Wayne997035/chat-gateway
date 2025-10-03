package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 應用程式配置結構.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
	Limits   LimitsConfig   `mapstructure:"limits"`
}

// AppConfig 應用程式基本配置.
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Debug   bool   `mapstructure:"debug"`
}

// ServerConfig 伺服器配置.
type ServerConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Timeout  int    `mapstructure:"timeout"`
	UseHTTPS bool   `mapstructure:"use_https"`
	CertPath string `mapstructure:"cert_path"`
	KeyPath  string `mapstructure:"key_path"`
}

// GRPCConfig gRPC 配置.
type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// DatabaseConfig 資料庫配置.
type DatabaseConfig struct {
	Mongo MongoConfig `mapstructure:"mongo"`
}

// MongoConfig MongoDB 配置.
type MongoConfig struct {
	URL                    string `mapstructure:"url"`
	Database               string `mapstructure:"database"`
	Username               string `mapstructure:"username"`
	Password               string `mapstructure:"password"`
	MaxPoolSize            uint64 `mapstructure:"max_pool_size"`
	MinPoolSize            uint64 `mapstructure:"min_pool_size"`
	MaxConnIdleTime        int    `mapstructure:"max_conn_idle_time"`
	ConnectTimeout         int    `mapstructure:"connect_timeout"`
	ServerSelectionTimeout int    `mapstructure:"server_selection_timeout"`
	TLSEnabled             bool   `mapstructure:"tls_enabled"`
	TLSCAFile              string `mapstructure:"tls_ca_file"`
	TLSCertFile            string `mapstructure:"tls_cert_file"`
	TLSKeyFile             string `mapstructure:"tls_key_file"`
	TLSInsecureSkipVerify  bool   `mapstructure:"tls_insecure_skip_verify"`
}

// LogConfig 日誌配置.
type LogConfig struct {
	RotationTimeHours int `mapstructure:"rotation_time_hours"` // 日誌輪轉時間 (小時).
	MaxAgeDays        int `mapstructure:"max_age_days"`        // 日誌保留天數.
	MaxSizeMB         int `mapstructure:"max_size_mb"`         // 單個日誌檔案最大大小 (MB).
}

// SecurityConfig 安全配置.
type SecurityConfig struct {
	TLS            TLSConfig            `mapstructure:"tls"`
	Authentication AuthenticationConfig `mapstructure:"authentication"`
	Encryption     EncryptionConfig     `mapstructure:"encryption"`
	Audit          AuditConfig          `mapstructure:"audit"`
}

// TLSConfig TLS 配置.
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

// AuthenticationConfig 認證配置.
type AuthenticationConfig struct {
	JWTEnabled bool   `mapstructure:"jwt_enabled"`
	JWTSecret  string `mapstructure:"jwt_secret"`
	Expiration string `mapstructure:"expiration"`
}

// EncryptionConfig 加密配置.
type EncryptionConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Algorithm string `mapstructure:"algorithm"`
	KeyLength int    `mapstructure:"key_length"`
}

// AuditConfig 審計配置.
type AuditConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Level   string `mapstructure:"level"`
}

// LimitsConfig 限制配置.
type LimitsConfig struct {
	Request      RequestLimitsConfig    `mapstructure:"request"`
	RateLimiting RateLimitingConfig     `mapstructure:"rate_limiting"`
	SSE          SSELimitsConfig        `mapstructure:"sse"`
	Pagination   PaginationLimitsConfig `mapstructure:"pagination"`
	Room         RoomLimitsConfig       `mapstructure:"room"`
	Message      MessageLimitsConfig    `mapstructure:"message"`
	MongoDB      MongoDBLimitsConfig    `mapstructure:"mongodb"`
}

// RequestLimitsConfig 請求限制配置.
type RequestLimitsConfig struct {
	MaxBodySize        int64 `mapstructure:"max_body_size"`
	MaxMultipartMemory int64 `mapstructure:"max_multipart_memory"`
}

// RateLimitingConfig Rate Limiting 配置.
type RateLimitingConfig struct {
	Enabled          bool `mapstructure:"enabled"`
	DefaultPerMinute int  `mapstructure:"default_per_minute"`
	MessagesPerMin   int  `mapstructure:"messages_per_minute"`
	RoomsPerMin      int  `mapstructure:"rooms_per_minute"`
	SSEPerMin        int  `mapstructure:"sse_per_minute"`
	CleanupInterval  int  `mapstructure:"cleanup_interval_minutes"`
}

// SSELimitsConfig SSE 限制配置.
type SSELimitsConfig struct {
	MaxConnectionsPerIP   int `mapstructure:"max_connections_per_ip"`
	MaxTotalConnections   int `mapstructure:"max_total_connections"`
	MinConnectionInterval int `mapstructure:"min_connection_interval_seconds"`
	HeartbeatInterval     int `mapstructure:"heartbeat_interval_seconds"`
	CleanupInterval       int `mapstructure:"cleanup_interval_minutes"`
	InitialMessageFetch   int `mapstructure:"initial_message_fetch"`
	MessageChannelBuffer  int `mapstructure:"message_channel_buffer"`
}

// PaginationLimitsConfig 分頁限制配置.
type PaginationLimitsConfig struct {
	DefaultPageSize int `mapstructure:"default_page_size"`
	MaxPageSize     int `mapstructure:"max_page_size"`
	MaxHistorySize  int `mapstructure:"max_history_size"`
}

// RoomLimitsConfig 聊天室限制配置.
type RoomLimitsConfig struct {
	MaxMembers    int `mapstructure:"max_members"`
	MaxNameLength int `mapstructure:"max_name_length"`
}

// MessageLimitsConfig 訊息限制配置.
type MessageLimitsConfig struct {
	MaxLength     int `mapstructure:"max_length"`
	ChannelBuffer int `mapstructure:"channel_buffer"`
}

// MongoDBLimitsConfig MongoDB 查詢限制配置.
type MongoDBLimitsConfig struct {
	DefaultQueryLimit int `mapstructure:"default_query_limit"`
	MaxQueryLimit     int `mapstructure:"max_query_limit"`
	MaxHistoryLimit   int `mapstructure:"max_history_limit"`
	UserRoomsLimit    int `mapstructure:"user_rooms_limit"`
	MaxStreamMessages int `mapstructure:"max_stream_messages"`
}

var (
	config *Config
	// ENV 當前環境變數.
	ENV string = "local"
)

// Load 載入設定檔.
func Load(testCfg ...*Config) error {
	// 如果直接傳入配置（主要用於測試），設定並驗證
	if len(testCfg) > 0 && testCfg[0] != nil {
		config = testCfg[0]
		// 驗證配置
		if err := validateConfig(config); err != nil {
			return fmt.Errorf("配置驗證失敗: %w", err)
		}
		return nil
	}

	// 初始化 Viper
	v := viper.New()

	// 檢查是否有 CONFIG_PATH 環境變數
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		// 使用 CONFIG_PATH 指定的檔案
		v.SetConfigFile(configPath)
		// 從檔案名稱推斷環境
		baseName := filepath.Base(configPath)
		ENV = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	} else {
		// 使用預設的環境配置檔案
		v.SetConfigName(ENV)
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
	}

	// 讀取配置檔案
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("讀取配置檔案失敗: %w", err)
	}

	// 將配置綁定到結構體
	config = &Config{}
	if err := v.Unmarshal(config); err != nil {
		return fmt.Errorf("解析配置失敗: %w", err)
	}

	// 驗證配置
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("配置驗證失敗: %w", err)
	}

	return nil
}

// Get 取得設定.
func Get() *Config {
	return config
}

// SetEnv 設定環境.
func SetEnv(env string) {
	ENV = env
}

// GetEnv 取得當前環境.
func GetEnv() string {
	return ENV
}

// validateConfig 驗證配置的有效性
func validateConfig(cfg *Config) error {
	// 驗證應用程式配置
	if cfg.App.Name == "" {
		return fmt.Errorf("應用程式名稱不能為空")
	}
	if cfg.App.Version == "" {
		return fmt.Errorf("應用程式版本不能為空")
	}

	// 驗證伺服器配置
	if cfg.Server.Host == "" {
		return fmt.Errorf("伺服器主機不能為空")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("伺服器端口不能為空")
	}
	if cfg.Server.Timeout <= 0 {
		return fmt.Errorf("伺服器超時時間必須大於 0")
	}

	// 驗證資料庫配置
	if cfg.Database.Mongo.URL == "" {
		return fmt.Errorf("MongoDB URL 不能為空")
	}
	if cfg.Database.Mongo.Database == "" {
		return fmt.Errorf("MongoDB 資料庫名稱不能為空")
	}
	if cfg.Database.Mongo.MaxPoolSize == 0 {
		return fmt.Errorf("MongoDB 最大連接池大小必須大於 0")
	}
	if cfg.Database.Mongo.MinPoolSize > cfg.Database.Mongo.MaxPoolSize {
		return fmt.Errorf("MongoDB 最小連接池大小不能大於最大連接池大小")
	}

	// 驗證日誌配置
	if cfg.Log.RotationTimeHours <= 0 {
		return fmt.Errorf("日誌輪轉時間必須大於 0")
	}
	if cfg.Log.MaxAgeDays <= 0 {
		return fmt.Errorf("日誌保留天數必須大於 0")
	}
	if cfg.Log.MaxSizeMB <= 0 {
		return fmt.Errorf("日誌檔案最大大小必須大於 0")
	}

	return nil
}

// IsDebug 檢查是否為除錯模式
func IsDebug() bool {
	if config != nil {
		return config.App.Debug
	}
	return false
}

// GetServerAddr 取得伺服器地址
func GetServerAddr() string {
	if config != nil {
		return fmt.Sprintf("%s:%s", config.Server.Host, config.Server.Port)
	}
	return "localhost:8080"
}

// GetMongoURL 取得 MongoDB 連接字串
func GetMongoURL() string {
	if config != nil {
		return config.Database.Mongo.URL
	}
	return ""
}
