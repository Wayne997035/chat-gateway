package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	Level string `mapstructure:"level"` // 日誌級別: debug, info, warn, error.
}

// SecurityConfig 安全配置.
type SecurityConfig struct {
	TLS            TLSConfig            `mapstructure:"tls"`
	Authentication AuthenticationConfig `mapstructure:"authentication"`
	Encryption     EncryptionConfig     `mapstructure:"encryption"`
	Audit          AuditConfig          `mapstructure:"audit"`
	KeyRotation    KeyRotationConfig    `mapstructure:"key_rotation"`
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
	KEKKeyset string `mapstructure:"kek_keyset"` // Tink JSON keyset, base64-encoded (set via CRYPTO_KEK_KEYSET)
	MasterKey string `mapstructure:"master_key"` // Legacy 32-byte raw key, base64-encoded; retained for gcm: migration
}

// AuditConfig 審計配置.
type AuditConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Level   string `mapstructure:"level"`
}

// KeyRotationConfig 密鑰輪換策略配置.
type KeyRotationConfig struct {
	Enabled               bool `mapstructure:"enabled"`
	RotationIntervalHours int  `mapstructure:"rotation_interval_hours"`
	MaxKeyAgeDays         int  `mapstructure:"max_key_age_days"`
	KeepOldKeys           int  `mapstructure:"keep_old_keys"`
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
	v.SetConfigType("yaml")

	// 決定配置檔案路徑
	var filePath string
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		filePath = configPath
		// 從檔案名稱推斷環境
		baseName := filepath.Base(configPath)
		ENV = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	} else {
		filePath = filepath.Join("configs", ENV+".yaml")
	}

	// 讀取並展開環境變數
	// #nosec G304,G703 -- filePath 來自管理員設定的環境變數或固定的 ./configs/ 目錄，非使用者輸入
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("讀取配置檔案失敗: %w", err)
	}
	expanded := os.ExpandEnv(string(raw))
	if err := v.ReadConfig(strings.NewReader(expanded)); err != nil {
		return fmt.Errorf("解析配置檔案失敗: %w", err)
	}

	// 將配置綁定到結構體
	config = &Config{}
	if err := v.Unmarshal(config); err != nil {
		return fmt.Errorf("解析配置失敗: %w", err)
	}

	// 從環境變數覆蓋 MongoDB 設定
	overrideMongoConfigFromEnv(config)

	// 從環境變數覆蓋安全設定
	overrideSecurityConfigFromEnv(config)

	// 從環境變數覆蓋密鑰輪換設定（並套用預設值）
	overrideKeyRotationFromEnv(&config.Security.KeyRotation)

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
	if err := validateAppConfig(cfg); err != nil {
		return err
	}
	if err := validateDatabaseConfig(cfg); err != nil {
		return err
	}
	if err := validateLogConfig(cfg); err != nil {
		return err
	}
	return validateSecurityConfig(cfg)
}

// validateAppConfig 驗證應用程式和伺服器配置
func validateAppConfig(cfg *Config) error {
	if cfg.App.Name == "" {
		return fmt.Errorf("應用程式名稱不能為空")
	}
	if cfg.App.Version == "" {
		return fmt.Errorf("應用程式版本不能為空")
	}
	if cfg.Server.Host == "" {
		return fmt.Errorf("伺服器主機不能為空")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("伺服器端口不能為空")
	}
	if cfg.Server.Timeout <= 0 {
		return fmt.Errorf("伺服器超時時間必須大於 0")
	}
	return nil
}

// validateDatabaseConfig 驗證資料庫配置
func validateDatabaseConfig(cfg *Config) error {
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
	return nil
}

// validateLogConfig 驗證日誌配置
func validateLogConfig(cfg *Config) error {
	level := cfg.Log.Level
	if level == "" {
		return nil
	}
	switch level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("log.level must be one of debug/info/warn/error, got %q", level)
	}
}

// validateSecurityConfig 驗證安全配置.
// 不論環境，以下規則恆成立：
//   - encryption.algorithm 若非空必須為 "AES-256-GCM"
//   - encryption.key_length 若非零必須為 256
//   - key_rotation.enabled=true 時，MaxKeyAgeDays*24 > RotationIntervalHours
//
// 僅在非 local 環境才強制檢查 credentials：
//   - encryption enabled → kek_keyset 必須非空
//   - jwt enabled → jwt_secret 必須非空
func validateSecurityConfig(cfg *Config) error {
	// 恆成立的規則（所有環境）
	if alg := cfg.Security.Encryption.Algorithm; alg != "" && alg != "AES-256-GCM" {
		return fmt.Errorf("security.encryption.algorithm must be \"AES-256-GCM\", got %q", alg)
	}
	if kl := cfg.Security.Encryption.KeyLength; kl != 0 && kl != 256 {
		return fmt.Errorf("security.encryption.key_length must be 256, got %d", kl)
	}
	if kr := cfg.Security.KeyRotation; kr.Enabled {
		if kr.RotationIntervalHours <= 0 {
			return fmt.Errorf("security.key_rotation.rotation_interval_hours must be > 0 when enabled")
		}
		if kr.MaxKeyAgeDays*24 <= kr.RotationIntervalHours {
			return fmt.Errorf(
				"security.key_rotation: max_key_age_days*24 (%d) must be greater than rotation_interval_hours (%d)",
				kr.MaxKeyAgeDays*24, kr.RotationIntervalHours,
			)
		}
	}

	// Credential checks — skipped in local env
	if ENV == "local" {
		return nil
	}
	if cfg.Security.Encryption.Enabled && cfg.Security.Encryption.KEKKeyset == "" {
		return fmt.Errorf("security.encryption.kek_keyset is required when encryption is enabled (set CRYPTO_KEK_KEYSET env var)")
	}
	if cfg.Security.Authentication.JWTEnabled && cfg.Security.Authentication.JWTSecret == "" {
		return fmt.Errorf("security.authentication.jwt_secret is required when JWT is enabled (set JWT_SECRET env var)")
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

// overrideMongoConfigFromEnv 從環境變數覆蓋 MongoDB 設定
func overrideMongoConfigFromEnv(cfg *Config) {
	// MongoDB URL（按優先順序檢查多種命名格式）
	if mongoURL := os.Getenv("MONGODB_URI"); mongoURL != "" {
		cfg.Database.Mongo.URL = mongoURL
		log.Printf("[MongoDB] 使用環境變數設定: %s", maskMongoURL(mongoURL)) //#nosec G706
	} else if mongoURL := os.Getenv("MONGO_URL"); mongoURL != "" {
		cfg.Database.Mongo.URL = mongoURL
		log.Printf("[MongoDB] 使用環境變數設定: %s", maskMongoURL(mongoURL)) //#nosec G706
	} else if mongoURL := os.Getenv("mongoURL"); mongoURL != "" {
		cfg.Database.Mongo.URL = mongoURL
		log.Printf("[MongoDB] 使用環境變數設定: %s", maskMongoURL(mongoURL)) //#nosec G706
	}

	// MongoDB 資料庫名稱
	if mongoDB := os.Getenv("MONGO_DATABASE"); mongoDB != "" {
		cfg.Database.Mongo.Database = mongoDB
	}

	// MongoDB 使用者名稱
	if mongoUsername := os.Getenv("MONGO_USERNAME"); mongoUsername != "" {
		cfg.Database.Mongo.Username = mongoUsername
	}

	// MongoDB 密碼
	if mongoPassword := os.Getenv("MONGO_PASSWORD"); mongoPassword != "" {
		cfg.Database.Mongo.Password = mongoPassword
	}

	// TLS 相關設定
	if tlsEnabled := os.Getenv("MONGO_TLS_ENABLED"); tlsEnabled != "" {
		cfg.Database.Mongo.TLSEnabled = tlsEnabled == "true" || tlsEnabled == "1"
	}

	if tlsCAFile := os.Getenv("MONGO_TLS_CA_FILE"); tlsCAFile != "" {
		cfg.Database.Mongo.TLSCAFile = tlsCAFile
	}

	if tlsCertFile := os.Getenv("MONGO_TLS_CERT_FILE"); tlsCertFile != "" {
		cfg.Database.Mongo.TLSCertFile = tlsCertFile
	}

	if tlsKeyFile := os.Getenv("MONGO_TLS_KEY_FILE"); tlsKeyFile != "" {
		cfg.Database.Mongo.TLSKeyFile = tlsKeyFile
	}
}

// overrideSecurityConfigFromEnv 從環境變數覆蓋安全設定.
func overrideSecurityConfigFromEnv(cfg *Config) {
	if ks := os.Getenv("CRYPTO_KEK_KEYSET"); ks != "" {
		cfg.Security.Encryption.KEKKeyset = ks
	}

	if mk := os.Getenv("MASTER_KEY"); mk != "" {
		cfg.Security.Encryption.MasterKey = mk
	}
}

// overrideKeyRotationFromEnv 從環境變數覆蓋密鑰輪換設定，並在未設定時套用預設值.
// development.yaml 使用純 ${VAR} 語法（os.ExpandEnv 不支援 ${VAR:default}），
// 因此預設值由此函數提供。
func overrideKeyRotationFromEnv(kr *KeyRotationConfig) {
	if v := os.Getenv("KEY_ROTATION_ENABLED"); v != "" {
		kr.Enabled = v == "true"
	}
	if v := os.Getenv("KEY_ROTATION_INTERVAL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			kr.RotationIntervalHours = n
		}
	} else if kr.RotationIntervalHours == 0 {
		kr.RotationIntervalHours = 24
	}
	if v := os.Getenv("KEY_ROTATION_MAX_AGE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			kr.MaxKeyAgeDays = n
		}
	} else if kr.MaxKeyAgeDays == 0 {
		kr.MaxKeyAgeDays = 30
	}
	if v := os.Getenv("KEY_ROTATION_KEEP_OLD_KEYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			kr.KeepOldKeys = n
		}
	} else if kr.KeepOldKeys == 0 {
		kr.KeepOldKeys = 5
	}
}

// maskMongoURL 遮蔽 MongoDB URL 中的敏感資訊
func maskMongoURL(url string) string {
	if url == "" {
		return "(空)"
	}

	// 遮蔽密碼部分，保留結構以便除錯
	// 例如: mongodb://user:password@host:port/db -> mongodb://user:***@host:port/db
	re := regexp.MustCompile(`(mongodb(?:\+srv)?://[^:]+:)([^@]+)(@.+)`)
	masked := re.ReplaceAllString(url, `$1***$3`)

	if masked == url {
		// 如果沒有密碼，顯示完整 URL
		return url
	}

	return masked
}
