package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestKeyRotationConfig_Unmarshal verifies that KeyRotationConfig fields unmarshal
// correctly from a viper-loaded YAML configuration.
func TestKeyRotationConfig_Unmarshal(t *testing.T) {
	t.Parallel()

	yaml := `
security:
  key_rotation:
    enabled: true
    rotation_interval_hours: 12
    max_key_age_days: 15
    keep_old_keys: 3
`

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("viper.ReadConfig: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("v.Unmarshal: %v", err)
	}

	kr := cfg.Security.KeyRotation
	if !kr.Enabled {
		t.Errorf("Enabled: got false, want true")
	}
	if kr.RotationIntervalHours != 12 {
		t.Errorf("RotationIntervalHours: got %d, want 12", kr.RotationIntervalHours)
	}
	if kr.MaxKeyAgeDays != 15 {
		t.Errorf("MaxKeyAgeDays: got %d, want 15", kr.MaxKeyAgeDays)
	}
	if kr.KeepOldKeys != 3 {
		t.Errorf("KeepOldKeys: got %d, want 3", kr.KeepOldKeys)
	}
}

// TestKeyRotationConfig_Defaults verifies that unset fields default to zero values.
func TestKeyRotationConfig_Defaults(t *testing.T) {
	t.Parallel()

	yaml := `
security:
  key_rotation: {}
`

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("viper.ReadConfig: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("v.Unmarshal: %v", err)
	}

	kr := cfg.Security.KeyRotation
	if kr.Enabled {
		t.Errorf("Enabled: got true, want false (zero)")
	}
	if kr.RotationIntervalHours != 0 {
		t.Errorf("RotationIntervalHours: got %d, want 0", kr.RotationIntervalHours)
	}
}

// ---------------------------------------------------------------------------
// validateSecurityConfig tests
// ---------------------------------------------------------------------------

func baseValidConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:    "test",
			Version: "1.0",
		},
		Server: ServerConfig{
			Host:    "localhost",
			Port:    "8080",
			Timeout: 30,
		},
		Database: DatabaseConfig{
			Mongo: MongoConfig{
				URL:         "mongodb://localhost:27017",
				Database:    "test",
				MaxPoolSize: 10,
			},
		},
		Log: LogConfig{
			Level: "info",
		},
		Security: SecurityConfig{
			Encryption: EncryptionConfig{
				Enabled:   true,
				Algorithm: "AES-256-GCM",
				KeyLength: 256,
				KEKKeyset: "some-keyset",
			},
			Authentication: AuthenticationConfig{
				JWTEnabled: false,
			},
		},
	}
}

// TestValidateSecurityConfig_AlgorithmMismatch verifies wrong algorithm is rejected in non-local env.
func TestValidateSecurityConfig_AlgorithmMismatch(t *testing.T) {
	t.Parallel()

	origENV := ENV
	ENV = "development"
	t.Cleanup(func() { ENV = origENV })

	cfg := baseValidConfig()
	cfg.Security.Encryption.Algorithm = "DES" // wrong

	err := validateSecurityConfig(cfg)
	if err == nil {
		t.Fatal("expected error for wrong algorithm, got nil")
	}
	if !strings.Contains(err.Error(), "AES-256-GCM") {
		t.Errorf("error should mention AES-256-GCM, got: %v", err)
	}
}

// TestValidateSecurityConfig_KeyLengthMismatch verifies wrong key length is rejected.
func TestValidateSecurityConfig_KeyLengthMismatch(t *testing.T) {
	t.Parallel()

	origENV := ENV
	ENV = "development"
	t.Cleanup(func() { ENV = origENV })

	cfg := baseValidConfig()
	cfg.Security.Encryption.KeyLength = 128 // wrong

	err := validateSecurityConfig(cfg)
	if err == nil {
		t.Fatal("expected error for wrong key length, got nil")
	}
	if !strings.Contains(err.Error(), "256") {
		t.Errorf("error should mention 256, got: %v", err)
	}
}

// TestValidateSecurityConfig_InvalidRotationInterval verifies rotation_interval_hours <= 0 and
// MaxKeyAgeDays*24 <= RotationIntervalHours are both rejected when key rotation is enabled.
func TestValidateSecurityConfig_InvalidRotationInterval(t *testing.T) {
	t.Parallel()

	origENV := ENV
	ENV = "development"
	t.Cleanup(func() { ENV = origENV })

	tests := []struct {
		name    string
		kr      KeyRotationConfig
		wantMsg string
	}{
		{
			name: "max_age_less_than_interval",
			kr: KeyRotationConfig{
				Enabled:               true,
				RotationIntervalHours: 48,
				MaxKeyAgeDays:         1, // 1*24 = 24 < 48: invalid
				KeepOldKeys:           5,
			},
			wantMsg: "max_key_age_days",
		},
		{
			name: "interval_hours_is_zero",
			kr: KeyRotationConfig{
				Enabled:               true,
				RotationIntervalHours: 0, // must be > 0
				MaxKeyAgeDays:         30,
				KeepOldKeys:           5,
			},
			wantMsg: "rotation_interval_hours",
		},
		{
			name: "interval_hours_is_negative",
			kr: KeyRotationConfig{
				Enabled:               true,
				RotationIntervalHours: -1,
				MaxKeyAgeDays:         30,
				KeepOldKeys:           5,
			},
			wantMsg: "rotation_interval_hours",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := baseValidConfig()
			cfg.Security.KeyRotation = tc.kr

			err := validateSecurityConfig(cfg)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error should contain %q, got: %v", tc.wantMsg, err)
			}
		})
	}
}

// TestValidateSecurityConfig_LocalEnvSkipsCredentials verifies local env passes even without KEK.
func TestValidateSecurityConfig_LocalEnvSkipsCredentials(t *testing.T) {
	t.Parallel()

	origENV := ENV
	ENV = "local"
	t.Cleanup(func() { ENV = origENV })

	cfg := baseValidConfig()
	cfg.Security.Encryption.KEKKeyset = "" // missing — OK in local
	cfg.Security.Encryption.Algorithm = "AES-256-GCM"
	cfg.Security.Encryption.KeyLength = 256

	err := validateSecurityConfig(cfg)
	if err != nil {
		t.Errorf("local env should pass without KEK, got: %v", err)
	}
}

// TestValidateLogConfig_InvalidLevel verifies invalid log level is rejected.
func TestValidateLogConfig_InvalidLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"", false},        // empty level: treat as valid (use default)
		{"verbose", true},  // unsupported level
		{"critical", true}, // unsupported level
		{"WARNING", true},  // case-sensitive
	}

	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			t.Parallel()

			cfg := baseValidConfig()
			cfg.Log.Level = tc.level

			err := validateLogConfig(cfg)
			if tc.wantErr && err == nil {
				t.Errorf("level %q: expected error, got nil", tc.level)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("level %q: expected nil, got: %v", tc.level, err)
			}
		})
	}
}
