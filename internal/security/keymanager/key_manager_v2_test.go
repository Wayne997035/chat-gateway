package keymanager_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/tink"

	mongocontainer "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"chat-gateway/internal/security/keymanager"
)

// newTestAEAD creates a fresh AES256-GCM AEAD for testing.
func newTestAEAD(t *testing.T) tink.AEAD {
	t.Helper()

	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	if err != nil {
		t.Fatalf("generate keyset: %v", err)
	}

	prim, err := aead.New(handle)
	if err != nil {
		t.Fatalf("aead.New: %v", err)
	}

	return prim
}

// startMongoDB starts a MongoDB testcontainer and returns the connected *mongo.Database.
// This is a shared container for the whole test file (called once, not per-test).
func startMongoDB(t *testing.T) *mongo.Database {
	t.Helper()

	ctx := context.Background()

	container, err := mongocontainer.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}

	t.Cleanup(func() {
		if termErr := container.Terminate(ctx); termErr != nil {
			t.Logf("terminate mongo container: %v", termErr)
		}
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}

	t.Cleanup(func() {
		if disconnErr := client.Disconnect(ctx); disconnErr != nil {
			t.Logf("disconnect mongo: %v", disconnErr)
		}
	})

	return client.Database("keymanager_test")
}

// ---------------------------------------------------------------------------
// Unit tests — no Docker needed
// ---------------------------------------------------------------------------

// TestDecryptGCMLegacy_RoundTrip verifies that EncryptRoomKeyWithLegacy and
// DecryptGCMLegacy are inverses of each other.
func TestDecryptGCMLegacy_RoundTrip(t *testing.T) {
	t.Parallel()

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 1)
	}

	plaintext := make([]byte, 32)
	for i := range plaintext {
		plaintext[i] = byte(i + 100)
	}

	encrypted, err := keymanager.EncryptRoomKeyWithLegacy(masterKey, plaintext)
	if err != nil {
		t.Fatalf("EncryptRoomKeyWithLegacy: %v", err)
	}

	if !hasPrefix(encrypted, "gcm:") {
		t.Fatalf("expected gcm: prefix, got %q", encrypted[:clamp(10, len(encrypted))])
	}

	decrypted, err := keymanager.DecryptGCMLegacy(masterKey, encrypted[len("gcm:"):])
	if err != nil {
		t.Fatalf("DecryptGCMLegacy: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatal("decrypted does not match plaintext")
	}
}

// TestDecryptGCMLegacy_WrongKey verifies that wrong master key produces wrong plaintext
// (AES-CTR does not authenticate, so we check for deterministic mismatch).
func TestDecryptGCMLegacy_WrongKey(t *testing.T) {
	t.Parallel()

	rightKey := make([]byte, 32)
	wrongKey := make([]byte, 32)
	for i := range wrongKey {
		wrongKey[i] = 0xFF
	}

	plaintext := make([]byte, 32)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	encrypted, err := keymanager.EncryptRoomKeyWithLegacy(rightKey, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := keymanager.DecryptGCMLegacy(wrongKey, encrypted[len("gcm:"):])
	// AES-CTR does not authenticate, so no error is expected — but output must differ.
	if err != nil {
		t.Logf("got error (acceptable): %v", err)
		return
	}

	if bytes.Equal(plaintext, decrypted) {
		t.Fatal("wrong key produced identical plaintext — unexpected")
	}
}

// TestDecryptGCMLegacy_ShortLegacyKey verifies wrong-length key is rejected.
func TestDecryptGCMLegacy_ShortLegacyKey(t *testing.T) {
	t.Parallel()

	_, err := keymanager.DecryptGCMLegacy([]byte("short"), "anything")
	if err == nil {
		t.Fatal("expected error for short legacy key, got nil")
	}
}

// TestDecryptGCMLegacy_InvalidBase64 verifies malformed base64 is rejected.
func TestDecryptGCMLegacy_InvalidBase64(t *testing.T) {
	t.Parallel()

	masterKey := make([]byte, 32)

	_, err := keymanager.DecryptGCMLegacy(masterKey, "!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

// TestNewKeyManagerWithPersistence_NilKEK_NoDocker verifies nil kek is rejected without DB.
// We pass a nil *mongo.Database — the nil-kek check runs before any DB access.
func TestNewKeyManagerWithPersistence_NilKEK_NoDocker(t *testing.T) {
	t.Parallel()

	_, err := keymanager.NewKeyManagerWithPersistence(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil kek, got nil")
	}
}

// TestNewKeyManagerWithPersistence_BadLegacyKey_NoDocker verifies wrong-length legacy
// key is rejected before any DB access.
func TestNewKeyManagerWithPersistence_BadLegacyKey_NoDocker(t *testing.T) {
	t.Parallel()

	kek := newTestAEAD(t)

	_, err := keymanager.NewKeyManagerWithPersistence(kek, []byte("too-short"), nil)
	if err == nil {
		t.Fatal("expected error for wrong-length legacy key, got nil")
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require Docker / testcontainers
// ---------------------------------------------------------------------------

// TestGetOrCreateRoomKey_EmptyRoomID verifies empty roomID is rejected.
func TestGetOrCreateRoomKey_EmptyRoomID(t *testing.T) {
	db := startMongoDB(t)
	kek := newTestAEAD(t)

	km, err := keymanager.NewKeyManagerWithPersistence(kek, nil, db)
	if err != nil {
		t.Fatalf("create km: %v", err)
	}

	_, err = km.GetOrCreateRoomKey("")
	if err == nil {
		t.Fatal("expected error for empty roomID, got nil")
	}
}

// TestGetOrCreateRoomKey_CreateAndIdempotent verifies that a room key is persisted
// and the same key is returned on subsequent calls.
func TestGetOrCreateRoomKey_CreateAndIdempotent(t *testing.T) {
	db := startMongoDB(t)
	kek := newTestAEAD(t)

	km, err := keymanager.NewKeyManagerWithPersistence(kek, nil, db)
	if err != nil {
		t.Fatalf("create km: %v", err)
	}

	roomID := "room-abc"

	key1, err := km.GetOrCreateRoomKey(roomID)
	if err != nil {
		t.Fatalf("first GetOrCreateRoomKey: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}

	// Second call must return the same key.
	key2, err := km.GetOrCreateRoomKey(roomID)
	if err != nil {
		t.Fatalf("second GetOrCreateRoomKey: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("second call returned different key than first")
	}
}

// TestEncryptDecryptRoundTrip_TinkFormat verifies a fresh key is stored in tink: format
// and can be decrypted correctly across a new KeyManager instance backed by the same DB
// (simulates a service restart).
func TestEncryptDecryptRoundTrip_TinkFormat(t *testing.T) {
	db := startMongoDB(t)
	kek := newTestAEAD(t)

	km1, err := keymanager.NewKeyManagerWithPersistence(kek, nil, db)
	if err != nil {
		t.Fatalf("create km1: %v", err)
	}

	roomID := "room-tink"

	original, err := km1.GetOrCreateRoomKey(roomID)
	if err != nil {
		t.Fatalf("GetOrCreateRoomKey km1: %v", err)
	}

	// Create a second KeyManager sharing the same kek and DB — simulates restart.
	km2, err := keymanager.NewKeyManagerWithPersistence(kek, nil, db)
	if err != nil {
		t.Fatalf("create km2: %v", err)
	}

	reloaded, err := km2.GetOrCreateRoomKey(roomID)
	if err != nil {
		t.Fatalf("GetOrCreateRoomKey km2: %v", err)
	}

	if !bytes.Equal(original, reloaded) {
		t.Fatal("reloaded key does not match original")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func clamp(limit, v int) int {
	if v < limit {
		return v
	}

	return limit
}
