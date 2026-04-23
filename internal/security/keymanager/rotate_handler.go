package keymanager

import (
	"crypto/subtle"
	"net/http"
	"regexp"

	"chat-gateway/internal/httputil"

	"github.com/gin-gonic/gin"
)

// mongoObjectIDRe matches a 24-character lowercase/uppercase hex string (MongoDB ObjectID format).
var mongoObjectIDRe = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// keyRotationManager is the interface KeyRotationHandler requires.
// Implemented by KeyManagerWithPersistence; also allows test doubles.
type keyRotationManager interface {
	GetOrCreateRoomKey(roomID string) ([]byte, error)
	ForceRotateKey(roomID string) error
	GetKeyInfo(roomID string) (*KeyInfo, error)
}

// KeyRotationHandler 密鑰輪換 HTTP handler.
type KeyRotationHandler struct {
	keyManager keyRotationManager
	adminToken string
}

// NewKeyRotationHandler 建立 KeyRotationHandler.
// adminToken 必須非空且長度 ≥ 32；不符合則 panic（應由 config.ValidateAdminToken 在啟動時攔截）.
func NewKeyRotationHandler(km *KeyManagerWithPersistence, adminToken string) *KeyRotationHandler {
	if adminToken == "" {
		panic("NewKeyRotationHandler: adminToken must not be empty")
	}
	if len(adminToken) < 32 {
		panic("NewKeyRotationHandler: adminToken must be at least 32 characters")
	}
	return &KeyRotationHandler{keyManager: km, adminToken: adminToken}
}

// ForceRotate handles POST /admin/rooms/:roomID/rotate-key.
// 使用 constant-time 比對防止 timing attack.
func (h *KeyRotationHandler) ForceRotate(c *gin.Context) {
	// 1. Validate Bearer token
	authHeader := c.GetHeader("Authorization")
	const bearerPrefix = "Bearer "
	if len(authHeader) <= len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		httputil.Unauthorized(c, "missing or invalid Authorization header")
		return
	}
	token := authHeader[len(bearerPrefix):]

	// constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.adminToken)) != 1 {
		httputil.Unauthorized(c, "invalid admin token")
		return
	}

	// 2. Validate roomID — must be a MongoDB ObjectID (24 hex chars)
	roomID := c.Param("roomID")
	if !mongoObjectIDRe.MatchString(roomID) {
		httputil.BadRequest(c, "roomID must be a valid 24-character hex ObjectID")
		return
	}

	// 3. Pre-load key into memory (ForceRotateKey requires key to be in km.keys)
	if _, err := h.keyManager.GetOrCreateRoomKey(roomID); err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	// 4. Force rotate
	if err := h.keyManager.ForceRotateKey(roomID); err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	// 5. Return new version
	info, err := h.keyManager.GetKeyInfo(roomID)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id":     roomID,
		"new_version": info.Version,
		"success":     true,
	})
}
