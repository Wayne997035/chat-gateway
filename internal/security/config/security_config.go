package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// SecurityConfig 安全配置 (遵循多項國際標準)
type SecurityConfig struct {
	// 加密配置 (PCI DSS 要求)
	Encryption EncryptionConfig `yaml:"encryption"`
	
	// 身份驗證配置 (ISO 27001 要求)
	Authentication AuthenticationConfig `yaml:"authentication"`
	
	// 授權配置 (ISO 27001 要求)
	Authorization AuthorizationConfig `yaml:"authorization"`
	
	// 審計配置 (PCI DSS, ISO 27001 要求)
	Audit AuditConfig `yaml:"audit"`
	
	// 數據保護配置 (GDPR 要求)
	DataProtection DataProtectionConfig `yaml:"data_protection"`
	
	// 網路安全配置 (ISO 27001 要求)
	NetworkSecurity NetworkSecurityConfig `yaml:"network_security"`
	
	// 合規性配置
	Compliance ComplianceConfig `yaml:"compliance"`
}

// EncryptionConfig 加密配置 (PCI DSS 要求)
type EncryptionConfig struct {
	// 對稱加密算法 (AES-256-GCM 符合 PCI DSS)
	SymmetricAlgorithm string `yaml:"symmetric_algorithm"`
	KeyLength          int    `yaml:"key_length"` // 256 bits
	
	// 非對稱加密算法 (RSA-4096 符合 PCI DSS)
	AsymmetricAlgorithm string `yaml:"asymmetric_algorithm"`
	RSABits             int    `yaml:"rsa_bits"` // 4096 bits
	
	// 密鑰管理 (PCI DSS 要求)
	KeyRotationInterval time.Duration `yaml:"key_rotation_interval"`
	KeyRetentionPeriod  time.Duration `yaml:"key_retention_period"`
	
	// 端到端加密 (Signal Protocol)
	E2EEncryption E2EEncryptionConfig `yaml:"e2e_encryption"`
}

// E2EEncryptionConfig 端到端加密配置
type E2EEncryptionConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Protocol          string        `yaml:"protocol"` // "signal" 或 "double_ratchet"
	KeyExchangeMethod string        `yaml:"key_exchange_method"`
	PerfectForwardSecrecy bool      `yaml:"perfect_forward_secrecy"`
	MessageRetention  time.Duration `yaml:"message_retention"`
}

// AuthenticationConfig 身份驗證配置 (ISO 27001 要求)
type AuthenticationConfig struct {
	// 多因素認證 (MFA)
	MFARequired bool `yaml:"mfa_required"`
	
	// 會話管理
	SessionTimeout     time.Duration `yaml:"session_timeout"`
	MaxConcurrentSessions int        `yaml:"max_concurrent_sessions"`
	
	// 密碼策略 (PCI DSS 要求)
	PasswordPolicy PasswordPolicy `yaml:"password_policy"`
	
	// JWT 配置
	JWT JWTConfig `yaml:"jwt"`
	
	// 生物識別 (如果支援)
	Biometric BiometricConfig `yaml:"biometric"`
}

// PasswordPolicy 密碼策略 (PCI DSS 要求)
type PasswordPolicy struct {
	MinLength        int           `yaml:"min_length"`
	MaxLength        int           `yaml:"max_length"`
	RequireUppercase bool          `yaml:"require_uppercase"`
	RequireLowercase bool          `yaml:"require_lowercase"`
	RequireNumbers   bool          `yaml:"require_numbers"`
	RequireSpecial   bool          `yaml:"require_special"`
	MaxAge           time.Duration `yaml:"max_age"`
	HistoryCount     int           `yaml:"history_count"`
	LockoutAttempts  int           `yaml:"lockout_attempts"`
	LockoutDuration  time.Duration `yaml:"lockout_duration"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey     string        `yaml:"secret_key"`
	Expiration    time.Duration `yaml:"expiration"`
	RefreshExpiration time.Duration `yaml:"refresh_expiration"`
	Issuer        string        `yaml:"issuer"`
	Audience      string        `yaml:"audience"`
}

// BiometricConfig 生物識別配置
type BiometricConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Fingerprint bool    `yaml:"fingerprint"`
	FaceID      bool    `yaml:"face_id"`
	VoiceID     bool    `yaml:"voice_id"`
	Threshold   float64 `yaml:"threshold"` // 識別閾值
}

// AuthorizationConfig 授權配置 (ISO 27001 要求)
type AuthorizationConfig struct {
	// 基於角色的訪問控制 (RBAC)
	RBACEnabled bool `yaml:"rbac_enabled"`
	
	// 基於屬性的訪問控制 (ABAC)
	ABACEnabled bool `yaml:"abac_enabled"`
	
	// 最小權限原則
	LeastPrivilege bool `yaml:"least_privilege"`
	
	// 權限檢查間隔
	PermissionCheckInterval time.Duration `yaml:"permission_check_interval"`
}

// AuditConfig 審計配置 (PCI DSS, ISO 27001 要求)
type AuditConfig struct {
	// 審計日誌級別
	LogLevel string `yaml:"log_level"` // DEBUG, INFO, WARN, ERROR
	
	// 審計事件
	Events AuditEvents `yaml:"events"`
	
	// 日誌存儲
	Storage AuditStorage `yaml:"storage"`
	
	// 日誌完整性
	Integrity AuditIntegrity `yaml:"integrity"`
	
	// 合規性報告
	ComplianceReporting ComplianceReporting `yaml:"compliance_reporting"`
}

// AuditEvents 審計事件配置
type AuditEvents struct {
	Authentication bool `yaml:"authentication"`
	Authorization  bool `yaml:"authorization"`
	DataAccess     bool `yaml:"data_access"`
	DataModification bool `yaml:"data_modification"`
	DataDeletion   bool `yaml:"data_deletion"`
	Configuration  bool `yaml:"configuration"`
	SecurityEvents bool `yaml:"security_events"`
}

// AuditStorage 審計存儲配置
type AuditStorage struct {
	Type           string        `yaml:"type"` // "file", "database", "syslog"
	RetentionPeriod time.Duration `yaml:"retention_period"`
	Encryption     bool          `yaml:"encryption"`
	Compression    bool          `yaml:"compression"`
	BackupEnabled  bool          `yaml:"backup_enabled"`
}

// AuditIntegrity 審計完整性配置
type AuditIntegrity struct {
	DigitalSignature bool   `yaml:"digital_signature"`
	HashAlgorithm    string `yaml:"hash_algorithm"` // SHA-256, SHA-512
	Timestamping     bool   `yaml:"timestamping"`
	ImmutableLogs    bool   `yaml:"immutable_logs"`
}

// ComplianceReporting 合規性報告配置
type ComplianceReporting struct {
	PCI_DSS   bool `yaml:"pci_dss"`
	ISO_27001 bool `yaml:"iso_27001"`
	SOC2      bool `yaml:"soc2"`
	GDPR      bool `yaml:"gdpr"`
}

// DataProtectionConfig 數據保護配置 (GDPR 要求)
type DataProtectionConfig struct {
	// 數據分類
	DataClassification bool `yaml:"data_classification"`
	
	// 數據加密
	EncryptionAtRest   bool `yaml:"encryption_at_rest"`
	EncryptionInTransit bool `yaml:"encryption_in_transit"`
	
	// 數據保留
	RetentionPolicy RetentionPolicy `yaml:"retention_policy"`
	
	// 數據刪除
	RightToErasure bool `yaml:"right_to_erasure"`
	
	// 數據可攜性
	DataPortability bool `yaml:"data_portability"`
	
	// 隱私影響評估
	PrivacyImpactAssessment bool `yaml:"privacy_impact_assessment"`
}

// RetentionPolicy 數據保留策略
type RetentionPolicy struct {
	DefaultRetention time.Duration `yaml:"default_retention"`
	MessageRetention time.Duration `yaml:"message_retention"`
	LogRetention     time.Duration `yaml:"log_retention"`
	AuditRetention   time.Duration `yaml:"audit_retention"`
}

// NetworkSecurityConfig 網路安全配置 (ISO 27001 要求)
type NetworkSecurityConfig struct {
	// TLS 配置
	TLS TLSConfig `yaml:"tls"`
	
	// 防火牆規則
	Firewall FirewallConfig `yaml:"firewall"`
	
	// DDoS 防護
	DDoSProtection DDoSProtectionConfig `yaml:"ddos_protection"`
	
	// 網路隔離
	NetworkSegmentation bool `yaml:"network_segmentation"`
}

// TLSConfig TLS 配置
type TLSConfig struct {
	MinVersion     string   `yaml:"min_version"`     // TLS 1.2
	MaxVersion     string   `yaml:"max_version"`     // TLS 1.3
	CipherSuites   []string `yaml:"cipher_suites"`
	CertFile       string   `yaml:"cert_file"`
	KeyFile        string   `yaml:"key_file"`
	ClientAuth     bool     `yaml:"client_auth"`
	ClientCAFile   string   `yaml:"client_ca_file"`
}

// FirewallConfig 防火牆配置
type FirewallConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedIPs     []string `yaml:"allowed_ips"`
	BlockedIPs     []string `yaml:"blocked_ips"`
	RateLimiting   bool     `yaml:"rate_limiting"`
	MaxConnections int      `yaml:"max_connections"`
}

// DDoSProtectionConfig DDoS 防護配置
type DDoSProtectionConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxRequestsPerMin int           `yaml:"max_requests_per_min"`
	BlockDuration     time.Duration `yaml:"block_duration"`
	WhitelistIPs      []string      `yaml:"whitelist_ips"`
}

// ComplianceConfig 合規性配置
type ComplianceConfig struct {
	// 合規性標準
	Standards []string `yaml:"standards"` // ["PCI_DSS", "ISO_27001", "SOC2", "GDPR"]
	
	// 合規性檢查
	ComplianceChecks ComplianceChecks `yaml:"compliance_checks"`
	
	// 風險評估
	RiskAssessment RiskAssessment `yaml:"risk_assessment"`
}

// ComplianceChecks 合規性檢查
type ComplianceChecks struct {
	AutomatedChecks bool          `yaml:"automated_checks"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	ReportGeneration bool         `yaml:"report_generation"`
}

// RiskAssessment 風險評估
type RiskAssessment struct {
	Enabled         bool          `yaml:"enabled"`
	AssessmentInterval time.Duration `yaml:"assessment_interval"`
	RiskThreshold   float64       `yaml:"risk_threshold"`
	MitigationPlan  bool          `yaml:"mitigation_plan"`
}

// NewSecurityConfig 創建默認安全配置
func NewSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Encryption: EncryptionConfig{
			SymmetricAlgorithm:  "AES-256-GCM",
			KeyLength:          256,
			AsymmetricAlgorithm: "RSA",
			RSABits:            4096,
			KeyRotationInterval: 24 * time.Hour,
			KeyRetentionPeriod:  365 * 24 * time.Hour,
			E2EEncryption: E2EEncryptionConfig{
				Enabled:              true,
				Protocol:             "signal",
				KeyExchangeMethod:    "X3DH",
				PerfectForwardSecrecy: true,
				MessageRetention:     30 * 24 * time.Hour,
			},
		},
		Authentication: AuthenticationConfig{
			MFARequired:           true,
			SessionTimeout:        15 * time.Minute,
			MaxConcurrentSessions: 3,
			PasswordPolicy: PasswordPolicy{
				MinLength:        12,
				MaxLength:        128,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSpecial:   true,
				MaxAge:           90 * 24 * time.Hour,
				HistoryCount:     12,
				LockoutAttempts:  5,
				LockoutDuration:  30 * time.Minute,
			},
			JWT: JWTConfig{
				Expiration:        15 * time.Minute,
				RefreshExpiration: 7 * 24 * time.Hour,
				Issuer:           "chat-gateway",
				Audience:         "chat-users",
			},
		},
		Authorization: AuthorizationConfig{
			RBACEnabled:              true,
			ABACEnabled:              true,
			LeastPrivilege:           true,
			PermissionCheckInterval:  5 * time.Minute,
		},
		Audit: AuditConfig{
			LogLevel: "INFO",
			Events: AuditEvents{
				Authentication:    true,
				Authorization:     true,
				DataAccess:        true,
				DataModification:  true,
				DataDeletion:      true,
				Configuration:     true,
				SecurityEvents:    true,
			},
			Storage: AuditStorage{
				Type:            "database",
				RetentionPeriod: 7 * 365 * 24 * time.Hour, // 7 年
				Encryption:      true,
				Compression:     true,
				BackupEnabled:   true,
			},
			Integrity: AuditIntegrity{
				DigitalSignature: true,
				HashAlgorithm:    "SHA-256",
				Timestamping:     true,
				ImmutableLogs:    true,
			},
			ComplianceReporting: ComplianceReporting{
				PCI_DSS:   true,
				ISO_27001: true,
				SOC2:      true,
				GDPR:      true,
			},
		},
		DataProtection: DataProtectionConfig{
			DataClassification:        true,
			EncryptionAtRest:          true,
			EncryptionInTransit:       true,
			RightToErasure:            true,
			DataPortability:           true,
			PrivacyImpactAssessment:   true,
			RetentionPolicy: RetentionPolicy{
				DefaultRetention: 365 * 24 * time.Hour,
				MessageRetention: 30 * 24 * time.Hour,
				LogRetention:     7 * 365 * 24 * time.Hour,
				AuditRetention:   7 * 365 * 24 * time.Hour,
			},
		},
		NetworkSecurity: NetworkSecurityConfig{
			TLS: TLSConfig{
				MinVersion:   "1.2",
				MaxVersion:   "1.3",
				ClientAuth:   true,
			},
			Firewall: FirewallConfig{
				Enabled:        true,
				RateLimiting:   true,
				MaxConnections: 1000,
			},
			DDoSProtection: DDoSProtectionConfig{
				Enabled:            true,
				MaxRequestsPerMin:  1000,
				BlockDuration:      5 * time.Minute,
			},
			NetworkSegmentation: true,
		},
		Compliance: ComplianceConfig{
			Standards: []string{"PCI_DSS", "ISO_27001", "SOC2", "GDPR"},
			ComplianceChecks: ComplianceChecks{
				AutomatedChecks:  true,
				CheckInterval:    24 * time.Hour,
				ReportGeneration: true,
			},
			RiskAssessment: RiskAssessment{
				Enabled:           true,
				AssessmentInterval: 7 * 24 * time.Hour,
				RiskThreshold:     0.7,
				MitigationPlan:    true,
			},
		},
	}
}

// GenerateRSAKeyPair 生成 RSA 密鑰對 (符合 PCI DSS 要求)
func (c *SecurityConfig) GenerateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, c.Encryption.RSABits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key pair: %v", err)
	}
	
	return privateKey, &privateKey.PublicKey, nil
}

// EncodePrivateKey 編碼私鑰為 PEM 格式
func (c *SecurityConfig) EncodePrivateKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	
	return privateKeyPEM, nil
}

// EncodePublicKey 編碼公鑰為 PEM 格式
func (c *SecurityConfig) EncodePublicKey(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}
	
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	
	return publicKeyPEM, nil
}
