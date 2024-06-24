/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"github.com/edgelesssys/estore/internal/base"
)

// GCMTagSize is the AES-GCM tag size.
const GCMTagSize = 16

var randomTestKey []byte

// GetCipher returns an AES-GCM cipher for key.
func GetCipher(key []byte) (cipher.AEAD, error) {
	// randomTestKey is set by TestEnableRandomKey. It's only
	// called in *_test.go, so is always nil in production.
	if len(key) == 0 && len(randomTestKey) == 16 {
		key = randomTestKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// TestEnableRandomKey enables the use of a random key in case no key has been set.
// This reduces required changes to existing tests.
func TestEnableRandomKey() {
	randomTestKey = make([]byte, 16)
	if _, err := rand.Read(randomTestKey); err != nil {
		panic(err)
	}
}

// EncryptOptions encrypts the contents of an OPTIONS file.
func EncryptOptions(
	serializedOpts []byte, fileNum base.DiskFileNum, keyManager *KeyManager,
) ([]byte, error) {
	key, err := keyManager.Create(fileNum.FileNum())
	if err != nil {
		return nil, err
	}
	aead, err := GetCipher(key)
	if err != nil {
		return nil, err
	}
	// Use all-zero nonce because the key is unique and won't be used again for another encryption.
	return aead.Seal(nil, make([]byte, aead.NonceSize()), serializedOpts, nil), nil
}

// DecryptOptions decrypts the contents of an OPTIONS file.
func DecryptOptions(
	ciphertext []byte, fileNum base.FileNum, keyManager *KeyManager,
) ([]byte, error) {
	key, err := keyManager.Get(fileNum)
	if err != nil {
		return nil, err
	}
	aead, err := GetCipher(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, make([]byte, aead.NonceSize()), ciphertext, nil)
}

// Writer is an interface for Write and WriteApproved.
type Writer interface {
	Write([]byte) (int, error)
	WriteApproved([]byte) (int, error)
}

// ApprovedWriter wraps an edg.Writer and provides an io.Writer interface.
// Use this to pass an edg.Writer to functions that expect an io.Writer.
type ApprovedWriter struct {
	Writer interface {
		WriteApproved([]byte) (int, error)
	}
}

func (w *ApprovedWriter) Write(p []byte) (int, error) {
	return w.Writer.WriteApproved(p)
}

// Discard is a Writer on which all Write calls succeed without doing anything.
var Discard Writer = discard{}

type discard struct{}

func (discard) Write(p []byte) (int, error) {
	return len(p), nil
}

func (discard) WriteApproved(p []byte) (int, error) {
	return len(p), nil
}
