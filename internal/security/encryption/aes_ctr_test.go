package encryption

import (
	"crypto/rand"
	"strings"
	"testing"
)

func TestAESCTREncryption(t *testing.T) {
	// 生成測試密鑰 (256 bits = 32 bytes)
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	
	enc, err := NewAESCTREncryption(key)
	if err != nil {
		t.Fatal(err)
	}
	
	testCases := []struct {
		name      string
		plaintext string
	}{
		{"Simple text", "Hello, World!"},
		{"Unicode", "你好世界！🔐"},
		{"Long text", strings.Repeat("This is a long message. ", 100)},
		{"Special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"Newlines", "Line 1\nLine 2\nLine 3"},
		{"Tabs", "Col1\tCol2\tCol3"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 測試加密
			ciphertext, err := enc.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}
			
			// 驗證格式
			if !strings.HasPrefix(ciphertext, "aes256ctr:") {
				t.Errorf("Invalid ciphertext format: missing prefix")
			}
			
			// 驗證加密後的內容不同
			if ciphertext == tc.plaintext {
				t.Errorf("Ciphertext should differ from plaintext")
			}
			
			// 測試解密
			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}
			
			// 驗證解密結果
			if decrypted != tc.plaintext {
				t.Errorf("Decryption mismatch.\nWant: %s\nGot: %s", tc.plaintext, decrypted)
			}
		})
	}
}

func TestAESCTREncryption_InvalidKey(t *testing.T) {
	testCases := []struct {
		name    string
		keySize int
	}{
		{"Too short", 16},  // 128 bits
		{"Too short", 24},  // 192 bits
		{"Too long", 48},   // 384 bits
		{"Empty", 0},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keySize)
			_, err := NewAESCTREncryption(key)
			if err == nil {
				t.Errorf("Expected error for key size %d, got nil", tc.keySize)
			}
		})
	}
}

func TestAESCTREncryption_WrongKey(t *testing.T) {
	// 創建兩個不同的密鑰
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)
	
	enc1, _ := NewAESCTREncryption(key1)
	enc2, _ := NewAESCTREncryption(key2)
	
	plaintext := "Secret message"
	
	// 用 key1 加密
	ciphertext, err := enc1.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	
	// 用 key2 解密（錯誤的密鑰）
	decrypted, err := enc2.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	
	// 解密應該得到亂碼（不應該等於原文）
	if decrypted == plaintext {
		t.Errorf("Wrong key should not decrypt to original plaintext")
	}
}

func TestAESCTREncryption_EmptyInput(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	// 測試空字符串加密
	_, err := enc.Encrypt("")
	if err == nil {
		t.Error("Expected error for empty plaintext")
	}
	
	// 測試空字符串解密
	_, err = enc.Decrypt("")
	if err == nil {
		t.Error("Expected error for empty ciphertext")
	}
}

func TestAESCTREncryption_InvalidFormat(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	testCases := []string{
		"no_prefix",
		"wrong_prefix:data",
		"aes256ctr:",           // 只有前綴
		"aes256ctr:invalid!!!",  // 無效 base64
		"aes256ctr:AA==",        // base64 有效但數據太短
	}
	
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := enc.Decrypt(tc)
			if err == nil {
				t.Errorf("Expected error for invalid format: %s", tc)
			}
		})
	}
}

func TestAESCTREncryption_DifferentIV(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	plaintext := "Same message"
	
	// 加密兩次同樣的訊息
	ciphertext1, _ := enc.Encrypt(plaintext)
	ciphertext2, _ := enc.Encrypt(plaintext)
	
	// 密文應該不同（因為 IV 不同）
	if ciphertext1 == ciphertext2 {
		t.Error("Same plaintext should produce different ciphertexts (different IVs)")
	}
	
	// 但解密後應該都是原文
	decrypted1, _ := enc.Decrypt(ciphertext1)
	decrypted2, _ := enc.Decrypt(ciphertext2)
	
	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both ciphertexts should decrypt to the same plaintext")
	}
}

func TestAESCTREncryption_Bytes(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	// 測試字節數據加密
	plaintext := []byte("Binary data: \x00\x01\x02\xff\xfe")
	
	ciphertext, err := enc.EncryptBytes(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	
	decrypted, err := enc.DecryptBytes(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(decrypted) != string(plaintext) {
		t.Error("Byte encryption/decryption mismatch")
	}
}

func TestAESCTREncryption_IsEncrypted(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	plaintext := "Test message"
	ciphertext, _ := enc.Encrypt(plaintext)
	
	if !enc.IsEncrypted(ciphertext) {
		t.Error("Should recognize encrypted text")
	}
	
	if enc.IsEncrypted(plaintext) {
		t.Error("Should not recognize plaintext as encrypted")
	}
	
	if enc.IsEncrypted("") {
		t.Error("Empty string should not be encrypted")
	}
}

func BenchmarkAESCTREncryption_Encrypt(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	plaintext := "This is a benchmark test message"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Encrypt(plaintext)
	}
}

func BenchmarkAESCTREncryption_Decrypt(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewAESCTREncryption(key)
	
	plaintext := "This is a benchmark test message"
	ciphertext, _ := enc.Encrypt(plaintext)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Decrypt(ciphertext)
	}
}

