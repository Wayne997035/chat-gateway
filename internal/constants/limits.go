package constants

// HTTP 請求相關常數
const (
	// 默認值（可被配置覆蓋）
	DefaultMaxRequestBodySize   = 10 << 20 // 10MB
	DefaultMaxMultipartMemory   = 10 << 20 // 10MB
	DefaultRequestTimeout       = 30       // 秒
)

// 分頁相關常數
const (
	DefaultPageSize        = 20
	DefaultMaxPageSize     = 100
	DefaultHistoryPageSize = 50
	MinPageSize            = 1
)

// 聊天室相關常數
const (
	DefaultMaxRoomMembers    = 1000
	DefaultMaxRoomNameLength = 100
	MinRoomNameLength        = 1
)

// 訊息相關常數
const (
	DefaultMaxMessageLength = 10000
	MessageChannelBuffer    = 10
)

// Rate Limiting 默認值
const (
	DefaultRateLimitPerMinute     = 100
	DefaultMessageRateLimit       = 30
	DefaultRoomCreateRateLimit    = 10
	DefaultSSERateLimit           = 5
	RateLimitCleanupIntervalMin   = 10 // 分鐘
)

// SSE 連接相關常數
const (
	DefaultSSEMaxConnectionsPerIP    = 3
	DefaultSSEMaxTotalConnections    = 1000
	DefaultSSEMinConnectionInterval  = 10  // 秒
	DefaultSSEHeartbeatInterval      = 15  // 秒
	SSEConnectionCleanupIntervalMin  = 10  // 分鐘
)

// 密鑰管理相關常數
const (
	DefaultKeyRotationIntervalHours = 24
	DefaultKeyMaxAgeDays            = 30
	DefaultKeepOldKeys              = 5
)

// MongoDB 查詢相關常數
const (
	DefaultMongoQueryLimit    = 20
	MaxMongoQueryLimit        = 100
	MaxMongoHistoryLimit      = 50
	DefaultUserRoomsLimit     = 100
	MaxStreamMessagesLimit    = 1000
)

// 用戶 ID 相關常數
const (
	MaxUserIDLength = 100
)

// 加密相關常數
const (
	EncryptedPrefixLength = 10
	MasterKeyLength       = 32 // 256 bits
)

