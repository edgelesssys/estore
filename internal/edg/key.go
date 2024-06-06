/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/edgelesssys/ego-kvstore/internal/base"
	"github.com/edgelesssys/ego-kvstore/vfs"
	"golang.org/x/crypto/hkdf"
)

const (
	// SaltChainFilename is the name of the salt chain file.
	SaltChainFilename = "SALTCHAIN"

	minKeySize    = 16
	saltBlockSize = fileNumSize + saltSize + macSize
	fileNumSize   = 8 // uint64
	saltSize      = 16
	macSize       = sha256.Size
)

// KeyManager manages the encryption keys for database files.
//
// Call Create(fileNum) to create a new key when writing a file.
// Call Get(fileNum) to get the key when reading a file.
//
// Internally, KeyManager maps file numbers to unique salts. File keys are derived with hkdf(masterKey, salt).
// The salts are stored integrity-protected in the SALTCHAIN file. The file is an append-only chain of
// saltBlocks, linked by HMACs.
//
// As the encrypted files are file-level integrity-protected, together with key management
// via the salt chain we achieve "snapshot integrity" for the entire database.
type KeyManager struct {
	masterKey []byte
	mu        sync.Mutex
	saltFile  vfs.File
	salts     map[base.FileNum][]byte
	lastMAC   []byte // MAC of the last written block
}

// TODO implement saltchain compaction

// NewKeyManager creates a new KeyManager.
func NewKeyManager(fs vfs.FS, dirname string, masterKey []byte) (*KeyManager, error) {
	if len(masterKey) < minKeySize && !(masterKey == nil && len(randomTestKey) == 16) {
		return nil, errors.New("invalid key size")
	}
	m := &KeyManager{
		masterKey: masterKey,
		salts:     map[base.FileNum][]byte{},
	}
	var err error
	m.saltFile, err = fs.OpenReadWrite(fs.PathJoin(dirname, SaltChainFilename))
	if err != nil {
		return nil, err
	}

	// read and verify existing SALTCHAIN
	for {
		// read block
		rawBlock := make([]byte, saltBlockSize)
		_, err := io.ReadFull(m.saltFile, rawBlock)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var block saltBlock
		if err := block.UnmarshalBinary(rawBlock); err != nil {
			return nil, err
		}

		// verify block's mac
		mac, err := m.hmac(block.fileNum, block.salt, m.lastMAC)
		if err != nil {
			return nil, err
		}
		if !hmac.Equal(mac, block.mac) {
			return nil, errors.New("invalid mac")
		}

		m.lastMAC = mac
		m.salts[block.fileNum] = block.salt
	}

	return m, nil
}

// Close closes the KeyManager.
func (m *KeyManager) Close() error {
	return m.saltFile.Close()
}

// Create creates a new key for writing a file.
func (m *KeyManager) Create(fileNum base.FileNum) ([]byte, error) {
	// prepare new block
	block := saltBlock{
		fileNum: fileNum,
		salt:    make([]byte, saltSize),
	}
	if _, err := rand.Read(block.salt); err != nil {
		return nil, err
	}

	// derive key
	key, err := m.derive(block.salt)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// calculate new block's mac
	block.mac, err = m.hmac(fileNum, block.salt, m.lastMAC)
	if err != nil {
		return nil, err
	}

	// write block
	rawBlock, err := block.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if _, err := m.saltFile.WriteApproved(rawBlock); err != nil {
		return nil, err
	}
	if err := m.saltFile.Sync(); err != nil {
		return nil, err
	}

	m.lastMAC = block.mac
	m.salts[fileNum] = block.salt

	return key, nil
}

// Get gets the key for reading a file.
func (m *KeyManager) Get(fileNum base.FileNum) ([]byte, error) {
	m.mu.Lock()
	salt, ok := m.salts[fileNum]
	m.mu.Unlock()
	if ok {
		return m.derive(salt)
	}
	if m.masterKey == nil && len(randomTestKey) == 16 {
		return randomTestKey, nil
	}
	return nil, errors.New("fileNum not found")
}

func (m *KeyManager) derive(salt []byte) ([]byte, error) {
	kdf := hkdf.New(sha256.New, m.masterKey, salt, nil)
	key := make([]byte, len(m.masterKey))
	if _, err := kdf.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (m *KeyManager) hmac(fileNum base.FileNum, salt []byte, previousMAC []byte) ([]byte, error) {
	data := binary.LittleEndian.AppendUint64(nil, uint64(fileNum))
	data = append(data, salt...)
	data = append(data, previousMAC...)
	mac := hmac.New(sha256.New, m.masterKey)
	if _, err := mac.Write(data); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

type saltBlock struct {
	fileNum base.FileNum
	salt    []byte
	mac     []byte // hmac(fileNum|salt|previousMAC)
}

func (b saltBlock) MarshalBinary() ([]byte, error) {
	data := binary.LittleEndian.AppendUint64(nil, uint64(b.fileNum))
	data = append(data, b.salt...)
	data = append(data, b.mac...)
	return data, nil
}

func (b *saltBlock) UnmarshalBinary(data []byte) error {
	if len(data) != saltBlockSize {
		return errors.New("invalid salt block size")
	}
	b.fileNum = base.FileNum(binary.LittleEndian.Uint64(data))
	b.salt = data[fileNumSize : fileNumSize+saltSize]
	b.mac = data[fileNumSize+saltSize:]
	return nil
}
