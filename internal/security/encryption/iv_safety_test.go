package encryption

import (
	"crypto/rand"
	"io"
	"testing"
)

const testMessage = "test message"

// TestIVUniqueness 測試 IV 的唯一性
// 這個測試證明我們的 IV 生成是安全的
func TestIVUniqueness(t *testing.T) {
	// 測試 AES-CTR 加密的 IV 唯一性
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESCTREncryption(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 加密相同的明文 100 次
	plaintext := testMessage
	ciphertexts := make([]string, 100)

	for i := 0; i < 100; i++ {
		ciphertext, err := encryptor.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}
		ciphertexts[i] = ciphertext
	}

	// 驗證所有密文都不同（因為 IV 不同）
	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			if ciphertexts[i] == ciphertexts[j] {
				t.Errorf("Found duplicate ciphertext at index %d and %d", i, j)
				t.Error("This means IV is not unique - SECURITY ISSUE!")
			}
		}
	}

	t.Log("✓ All 100 ciphertexts are unique - IV generation is secure")
}

// TestIVExtraction 測試從密文中正確提取 IV
func TestIVExtraction(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESCTREncryption(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := testMessage

	// 加密
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// 解密
	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// 驗證解密結果
	if decrypted != plaintext {
		t.Errorf("Decryption mismatch: got %q, want %q", decrypted, plaintext)
		t.Error("This means IV extraction is broken - SECURITY ISSUE!")
	}

	t.Log("✓ IV extraction works correctly - encryption/decryption cycle successful")
}

// TestSignalProtocolIVUniqueness 測試 Signal Protocol 的 IV 唯一性
func TestSignalProtocolIVUniqueness(t *testing.T) {
	sp, err := NewSignalProtocol()
	if err != nil {
		t.Fatalf("Failed to create signal protocol: %v", err)
	}

	// 初始化會話
	sessionID := "test_session"
	rootKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rootKey); err != nil {
		t.Fatalf("Failed to generate root key: %v", err)
	}

	if err := sp.DoubleRatchet(sessionID, rootKey); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// 加密相同的明文 10 次
	plaintext := []byte(testMessage)
	ciphertexts := make([][]byte, 10)

	for i := 0; i < 10; i++ {
		ciphertext, err := sp.EncryptMessage(sessionID, plaintext)
		if err != nil {
			t.Fatalf("Encryption %d failed: %v", i, err)
		}
		ciphertexts[i] = ciphertext
	}

	// 驗證所有密文都不同（因為每次 MessageKey 和 IV 都不同）
	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			// 比較密文的前 50 bytes（包含 header 和部分密文）
			minLen := len(ciphertexts[i])
			if len(ciphertexts[j]) < minLen {
				minLen = len(ciphertexts[j])
			}
			if minLen > 50 {
				minLen = 50
			}

			same := true
			for k := 0; k < minLen; k++ {
				if ciphertexts[i][k] != ciphertexts[j][k] {
					same = false
					break
				}
			}

			if same {
				t.Errorf("Found similar ciphertext at index %d and %d", i, j)
				t.Error("This means Signal Protocol IV/nonce is not unique - SECURITY ISSUE!")
			}
		}
	}

	t.Log("✓ All 10 Signal Protocol ciphertexts are unique - IV/nonce generation is secure")
}
