package kek

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/tink"
)

// Provider builds a tink.AEAD representing the KEK source.
// Currently implemented by EnvVarProvider (cleartext keyset in env var).
// Switching to Cloud KMS later only requires a new Provider implementation;
// all other code remains unchanged.
type Provider interface {
	AEAD() (tink.AEAD, error)
}

// EnvVarProvider creates an AEAD from a base64-encoded Tink JSON keyset string.
type EnvVarProvider struct {
	KeysetBase64 string
}

// AEAD loads the keyset and returns a tink.AEAD primitive.
func (p EnvVarProvider) AEAD() (tink.AEAD, error) {
	if p.KeysetBase64 == "" {
		return nil, fmt.Errorf("KEK keyset is empty")
	}

	keysetJSON, err := base64.StdEncoding.DecodeString(p.KeysetBase64)
	if err != nil {
		return nil, fmt.Errorf("decode KEK keyset: %w", err)
	}

	reader := keyset.NewJSONReader(bytes.NewReader(keysetJSON))

	handle, err := insecurecleartextkeyset.Read(reader)
	if err != nil {
		return nil, fmt.Errorf("load KEK keyset: %w", err)
	}

	primitive, err := aead.New(handle)
	if err != nil {
		return nil, fmt.Errorf("build KEK AEAD: %w", err)
	}

	return primitive, nil
}

// Future Cloud KMS provider (not yet implemented):
//
// type GCPKMSProvider struct {
//     KeyURI           string
//     EncryptedKeyset  []byte
// }
//
// func (p GCPKMSProvider) AEAD() (tink.AEAD, error) { ... gcpkms ... }
