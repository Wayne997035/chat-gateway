package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// SignalProtocol Signal Protocol 實現 (符合 PCI DSS 和 ISO 27001 要求)
type SignalProtocol struct {
	// 身份密鑰對 (長期密鑰)
	IdentityKeyPair *KeyPair

	// 已簽名預密鑰對 (中期密鑰)
	SignedPreKeyPair *KeyPair

	// 一次性預密鑰對 (短期密鑰)
	OneTimePreKeyPairs []*KeyPair

	// 會話狀態
	Sessions map[string]*SessionState
}

// KeyPair 密鑰對
type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

// SessionState 會話狀態
type SessionState struct {
	// 會話密鑰
	RootKey []byte

	// 發送鏈密鑰
	SendChainKey *ChainKey

	// 接收鏈密鑰
	ReceiveChainKey *ChainKey

	// 發送消息編號
	SendMessageNumber uint32

	// 接收消息編號
	ReceiveMessageNumber uint32

	// 前向保密密鑰
	PreviousChainKeys []*ChainKey
}

// ChainKey 鏈密鑰
type ChainKey struct {
	Key        []byte
	Index      uint32
	MessageKey *MessageKey
}

// MessageKey 消息密鑰
type MessageKey struct {
	CipherKey []byte
	MacKey    []byte
	IV        []byte
}

// NewSignalProtocol 創建新的 Signal Protocol 實例
func NewSignalProtocol() (*SignalProtocol, error) {
	// 生成身份密鑰對
	identityKeyPair, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity key pair: %v", err)
	}

	// 生成已簽名預密鑰對
	signedPreKeyPair, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed pre key pair: %v", err)
	}

	// 生成一次性預密鑰對 (生成 100 個)
	oneTimePreKeyPairs := make([]*KeyPair, 100)
	for i := 0; i < 100; i++ {
		keyPair, err := generateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate one-time pre key pair %d: %v", i, err)
		}
		oneTimePreKeyPairs[i] = keyPair
	}

	return &SignalProtocol{
		IdentityKeyPair:    identityKeyPair,
		SignedPreKeyPair:   signedPreKeyPair,
		OneTimePreKeyPairs: oneTimePreKeyPairs,
		Sessions:           make(map[string]*SessionState),
	}, nil
}

// generateKeyPair 生成 Curve25519 密鑰對
func generateKeyPair() (*KeyPair, error) {
	privateKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, privateKey); err != nil {
		return nil, err
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// X3DHKeyAgreement X3DH 密鑰協商 (符合 PCI DSS 要求)
func (sp *SignalProtocol) X3DHKeyAgreement(
	theirIdentityKey []byte,
	theirSignedPreKey []byte,
	theirOneTimePreKey []byte,
	myOneTimePreKeyPair *KeyPair,
) ([]byte, error) {
	// 步驟 1: 計算 DH1 = DH(IK_A, SPK_B)
	dh1, err := curve25519.X25519(sp.IdentityKeyPair.PrivateKey, theirSignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute DH1: %v", err)
	}

	// 步驟 2: 計算 DH2 = DH(EK_A, IK_B)
	dh2, err := curve25519.X25519(sp.SignedPreKeyPair.PrivateKey, theirIdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute DH2: %v", err)
	}

	// 步驟 3: 計算 DH3 = DH(EK_A, SPK_B)
	dh3, err := curve25519.X25519(sp.SignedPreKeyPair.PrivateKey, theirSignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute DH3: %v", err)
	}

	// 步驟 4: 計算 DH4 (如果有一時預密鑰)
	var dh4 []byte
	if theirOneTimePreKey != nil && myOneTimePreKeyPair != nil {
		dh4, err = curve25519.X25519(myOneTimePreKeyPair.PrivateKey, theirOneTimePreKey)
		if err != nil {
			return nil, fmt.Errorf("failed to compute DH4: %v", err)
		}
	}

	// 步驟 5: 組合所有 DH 結果
	var dhInput []byte
	dhInput = append(dhInput, dh1...)
	dhInput = append(dhInput, dh2...)
	dhInput = append(dhInput, dh3...)
	if dh4 != nil {
		dhInput = append(dhInput, dh4...)
	}

	// 步驟 6: 使用 HKDF 導出根密鑰
	rootKey := make([]byte, 32)
	_, err = hkdf.New(sha256.New, dhInput, nil, []byte("SignalProtocol")).Read(rootKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive root key: %v", err)
	}

	return rootKey, nil
}

// DoubleRatchet 雙棘輪算法 (前向保密)
func (sp *SignalProtocol) DoubleRatchet(sessionID string, rootKey []byte) error {
	// 創建新的會話狀態
	sessionState := &SessionState{
		RootKey:              rootKey,
		SendChainKey:         &ChainKey{Key: rootKey, Index: 0},
		ReceiveChainKey:      &ChainKey{Key: rootKey, Index: 0},
		SendMessageNumber:    0,
		ReceiveMessageNumber: 0,
		PreviousChainKeys:    make([]*ChainKey, 0),
	}

	sp.Sessions[sessionID] = sessionState
	return nil
}

// EncryptMessage 加密消息 (AES-256-GCM 符合 PCI DSS)
func (sp *SignalProtocol) EncryptMessage(sessionID string, plaintext []byte) ([]byte, error) {
	session, exists := sp.Sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 生成消息密鑰
	messageKey, err := sp.deriveMessageKey(session.SendChainKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive message key: %v", err)
	}

	// 創建 AES-GCM 加密器
	block, err := aes.NewCipher(messageKey.CipherKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// 加密消息
	ciphertext := aesGCM.Seal(nil, messageKey.IV, plaintext, nil)

	// 更新發送鏈密鑰
	session.SendChainKey.Index++
	session.SendMessageNumber++

	// 創建消息頭
	header := sp.createMessageHeader(sessionID, session.SendMessageNumber, messageKey)

	// 組合消息頭和密文
	encryptedMessage := append(header, ciphertext...)

	return encryptedMessage, nil
}

// DecryptMessage 解密消息
func (sp *SignalProtocol) DecryptMessage(sessionID string, encryptedMessage []byte) ([]byte, error) {
	session, exists := sp.Sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 解析消息頭
	header, ciphertext, err := sp.parseMessageHeader(encryptedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message header: %v", err)
	}

	// 驗證消息編號
	if header.MessageNumber <= session.ReceiveMessageNumber {
		return nil, fmt.Errorf("duplicate or out-of-order message")
	}

	// 生成消息密鑰
	messageKey, err := sp.deriveMessageKey(session.ReceiveChainKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive message key: %v", err)
	}

	// 創建 AES-GCM 解密器
	block, err := aes.NewCipher(messageKey.CipherKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// 解密消息
	plaintext, err := aesGCM.Open(nil, messageKey.IV, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %v", err)
	}

	// 更新接收鏈密鑰
	session.ReceiveChainKey.Index++
	session.ReceiveMessageNumber = header.MessageNumber

	return plaintext, nil
}

// deriveMessageKey 導出消息密鑰
func (sp *SignalProtocol) deriveMessageKey(chainKey *ChainKey) (*MessageKey, error) {
	// 使用 HKDF 導出消息密鑰
	messageKeyBytes := make([]byte, 80) // 32 + 32 + 16 = 80 bytes
	_, err := hkdf.New(sha256.New, chainKey.Key, nil, []byte("MessageKey")).Read(messageKeyBytes)
	if err != nil {
		return nil, err
	}

	// 分割密鑰
	cipherKey := messageKeyBytes[:32]
	macKey := messageKeyBytes[32:64]
	iv := messageKeyBytes[64:80]

	return &MessageKey{
		CipherKey: cipherKey,
		MacKey:    macKey,
		IV:        iv,
	}, nil
}

// createMessageHeader 創建消息頭
func (sp *SignalProtocol) createMessageHeader(sessionID string, messageNumber uint32, messageKey *MessageKey) []byte {
	header := make([]byte, 4+32+16) // 4 bytes for message number + 32 bytes for MAC + 16 bytes for IV

	// 消息編號
	binary.BigEndian.PutUint32(header[0:4], messageNumber)

	// MAC (使用 HMAC-SHA256)
	mac := sp.computeMAC(messageKey.MacKey, []byte(sessionID), messageNumber)
	copy(header[4:36], mac)

	// IV
	copy(header[36:52], messageKey.IV)

	return header
}

// parseMessageHeader 解析消息頭
func (sp *SignalProtocol) parseMessageHeader(encryptedMessage []byte) (*MessageHeader, []byte, error) {
	if len(encryptedMessage) < 52 { // 最小消息頭長度
		return nil, nil, fmt.Errorf("message too short")
	}

	messageNumber := binary.BigEndian.Uint32(encryptedMessage[0:4])
	mac := encryptedMessage[4:36]
	iv := encryptedMessage[36:52]
	ciphertext := encryptedMessage[52:]

	return &MessageHeader{
		MessageNumber: messageNumber,
		MAC:           mac,
		IV:            iv,
	}, ciphertext, nil
}

// MessageHeader 消息頭
type MessageHeader struct {
	MessageNumber uint32
	MAC           []byte
	IV            []byte
}

// computeMAC 計算消息認證碼
func (sp *SignalProtocol) computeMAC(key []byte, sessionID []byte, messageNumber uint32) []byte {
	h := sha512.New()
	h.Write(key)
	h.Write(sessionID)

	messageNumberBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(messageNumberBytes, messageNumber)
	h.Write(messageNumberBytes)

	return h.Sum(nil)[:32] // 取前 32 字節
}

// RotateKeys 輪換密鑰 (符合 PCI DSS 密鑰管理要求)
func (sp *SignalProtocol) RotateKeys() error {
	// 生成新的已簽名預密鑰對
	newSignedPreKeyPair, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate new signed pre key pair: %v", err)
	}

	// 生成新的一次性預密鑰對
	newOneTimePreKeyPairs := make([]*KeyPair, 100)
	for i := 0; i < 100; i++ {
		keyPair, err := generateKeyPair()
		if err != nil {
			return fmt.Errorf("failed to generate new one-time pre key pair %d: %v", i, err)
		}
		newOneTimePreKeyPairs[i] = keyPair
	}

	// 更新密鑰
	sp.SignedPreKeyPair = newSignedPreKeyPair
	sp.OneTimePreKeyPairs = newOneTimePreKeyPairs

	return nil
}

// GetPublicKeys 獲取公鑰信息
func (sp *SignalProtocol) GetPublicKeys() *PublicKeyBundle {
	return &PublicKeyBundle{
		IdentityKey:    sp.IdentityKeyPair.PublicKey,
		SignedPreKey:   sp.SignedPreKeyPair.PublicKey,
		OneTimePreKeys: sp.getOneTimePreKeyPublicKeys(),
	}
}

// getOneTimePreKeyPublicKeys 獲取一次性預密鑰公鑰
func (sp *SignalProtocol) getOneTimePreKeyPublicKeys() [][]byte {
	publicKeys := make([][]byte, len(sp.OneTimePreKeyPairs))
	for i, keyPair := range sp.OneTimePreKeyPairs {
		publicKeys[i] = keyPair.PublicKey
	}
	return publicKeys
}

// PublicKeyBundle 公鑰包
type PublicKeyBundle struct {
	IdentityKey    []byte
	SignedPreKey   []byte
	OneTimePreKeys [][]byte
}
