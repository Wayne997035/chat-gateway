package keymanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeKeyManager satisfies keyRotationManager for unit tests.
type fakeKeyManager struct {
	getOrCreateResult []byte
	getOrCreateErr    error
	forceRotateErr    error
	keyInfoResult     *KeyInfo
	keyInfoErr        error
}

func (f *fakeKeyManager) GetOrCreateRoomKey(_ string) ([]byte, error) {
	return f.getOrCreateResult, f.getOrCreateErr
}

func (f *fakeKeyManager) ForceRotateKey(_ string) error {
	return f.forceRotateErr
}

func (f *fakeKeyManager) GetKeyInfo(_ string) (*KeyInfo, error) {
	return f.keyInfoResult, f.keyInfoErr
}

// newFakeHandler builds a KeyRotationHandler backed by a fakeKeyManager.
func newFakeHandler(fake *fakeKeyManager, adminToken string) *KeyRotationHandler {
	return &KeyRotationHandler{keyManager: fake, adminToken: adminToken}
}

// newTestCtx creates a POST Gin context backed by an httptest.ResponseRecorder.
func newTestCtx(path, roomID string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, path, http.NoBody)
	c.Request = req
	c.Params = gin.Params{{Key: "roomID", Value: roomID}}
	return w, c
}

func TestForceRotate_HappyPath(t *testing.T) {
	const adminToken = "super-secret"
	const roomID = "507f1f77bcf86cd799439011" // valid MongoDB ObjectID
	const expectedVersion = 3

	fake := &fakeKeyManager{
		getOrCreateResult: []byte("key"),
		keyInfoResult:     &KeyInfo{RoomID: roomID, Version: expectedVersion, Status: KeyStatusActive},
	}
	h := newFakeHandler(fake, adminToken)

	w, c := newTestCtx("/admin/rooms/"+roomID+"/rotate-key", roomID)
	c.Request.Header.Set("Authorization", "Bearer "+adminToken)

	h.ForceRotate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["room_id"] != roomID {
		t.Errorf("room_id: got %v, want %s", resp["room_id"], roomID)
	}
	if resp["new_version"] != float64(expectedVersion) {
		t.Errorf("new_version: got %v, want %d", resp["new_version"], expectedVersion)
	}
	if resp["success"] != true {
		t.Errorf("success: got %v, want true", resp["success"])
	}
}

func TestForceRotate_AuthErrors(t *testing.T) {
	cases := []struct {
		name       string
		authHeader string
	}{
		{"no Authorization header", ""},
		{"wrong token", "Bearer wrong-token"},
		{"no Bearer prefix", "super-secret"},
		{"empty Bearer", "Bearer "},
	}

	const validRoomID = "507f1f77bcf86cd799439011"
	fake := &fakeKeyManager{getOrCreateResult: []byte("key")}
	h := newFakeHandler(fake, "super-secret")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newTestCtx("/admin/rooms/"+validRoomID+"/rotate-key", validRoomID)
			if tc.authHeader != "" {
				c.Request.Header.Set("Authorization", tc.authHeader)
			}

			h.ForceRotate(c)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestForceRotate_InvalidRoomID(t *testing.T) {
	fake := &fakeKeyManager{}
	h := newFakeHandler(fake, "token")

	w, c := newTestCtx("/admin/rooms/invalid/rotate-key", "invalid")
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ForceRotate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestForceRotate_GetOrCreateFails(t *testing.T) {
	const roomID = "507f1f77bcf86cd799439011"
	fake := &fakeKeyManager{
		getOrCreateErr: fmt.Errorf("room does not exist"),
	}
	h := newFakeHandler(fake, "token")

	w, c := newTestCtx("/admin/rooms/"+roomID+"/rotate-key", roomID)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ForceRotate(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestForceRotate_RotateFails(t *testing.T) {
	const roomID = "507f1f77bcf86cd799439011"
	fake := &fakeKeyManager{
		getOrCreateResult: []byte("key"),
		forceRotateErr:    fmt.Errorf("rotation error"),
	}
	h := newFakeHandler(fake, "token")

	w, c := newTestCtx("/admin/rooms/"+roomID+"/rotate-key", roomID)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ForceRotate(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestForceRotate_GetKeyInfoFails(t *testing.T) {
	const roomID = "507f1f77bcf86cd799439011"
	fake := &fakeKeyManager{
		getOrCreateResult: []byte("key"),
		keyInfoErr:        fmt.Errorf("key info unavailable"),
	}
	h := newFakeHandler(fake, "token")

	w, c := newTestCtx("/admin/rooms/"+roomID+"/rotate-key", roomID)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ForceRotate(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
