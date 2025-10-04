package keymanager

import "time"

// Key 加密密鑰
type Key struct {
	ID        string    // 密鑰 ID (通常是 roomID)
	Value     []byte    // 256-bit 密鑰值
	CreatedAt time.Time // 創建時間
	RotatedAt time.Time // 最後輪換時間
	Version   int       // 密鑰版本
	Status    KeyStatus // 密鑰狀態
}

// KeyStatus 密鑰狀態
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"   // 活躍（用於加密）
	KeyStatusArchived KeyStatus = "archived" // 歸檔（只用於解密）
	KeyStatusRevoked  KeyStatus = "revoked"  // 撤銷（不可用）
)

// RotationPolicy 密鑰輪換策略
type RotationPolicy struct {
	Enabled          bool          // 是否啟用自動輪換
	RotationInterval time.Duration // 輪換間隔
	MaxKeyAge        time.Duration // 密鑰最大使用時間
	KeepOldKeys      int           // 保留的歷史密鑰數量
}

// KeyInfo 密鑰信息（不包含密鑰值）
type KeyInfo struct {
	RoomID    string
	Version   int
	CreatedAt time.Time
	RotatedAt time.Time
	Status    KeyStatus
	Age       time.Duration
}

// KeyManagerStats 密鑰管理器統計信息
type KeyManagerStats struct {
	TotalKeys    int
	ActiveKeys   int
	ArchivedKeys int
	RevokedKeys  int
}
