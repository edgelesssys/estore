/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg

import (
	"bytes"
	"testing"

	"github.com/edgelesssys/estore/internal/base"
	"github.com/edgelesssys/estore/vfs"
	"github.com/stretchr/testify/require"
)

func TestKeyManager(t *testing.T) {
	require := require.New(t)
	fs := vfs.NewMem()
	masterKey := bytes.Repeat([]byte{2}, 16)

	requireGetError := func(km *KeyManager, nums ...base.FileNum) {
		for _, n := range nums {
			_, err := km.Get(n)
			require.Error(err)
		}
	}
	requireGet := func(km *KeyManager, num base.FileNum, key []byte) {
		keyGot, err := km.Get(num)
		require.NoError(err)
		require.Equal(key, keyGot)
	}
	requireCreate := func(km *KeyManager, num base.FileNum) []byte {
		key, err := km.Create(num)
		require.NoError(err)
		require.Len(key, len(masterKey))
		return key
	}

	// File doesn't exist yet, require Get to return error for all file numbers
	km, err := NewKeyManager(fs, "", masterKey)
	require.NoError(err)
	requireGetError(km, 0, 1, 2, 3, 4, 5)
	require.NoError(km.Close())

	// File is empty, require Get to return error for all file numbers
	km, err = NewKeyManager(fs, "", masterKey)
	require.NoError(err)
	requireGetError(km, 0, 1, 2, 3, 4, 5)
	// Create key for file number 2
	key2 := requireCreate(km, 2)
	requireGetError(km, 0, 1, 3, 4, 5)
	requireGet(km, 2, key2)
	require.NoError(km.Close())

	// Key 2 can be retrieved after reopening the file
	km, err = NewKeyManager(fs, "", masterKey)
	require.NoError(err)
	requireGetError(km, 0, 1, 3, 4, 5)
	requireGet(km, 2, key2)
	// Create key for file number 4
	key4 := requireCreate(km, 4)
	require.NotEqual(key2, key4)
	requireGetError(km, 0, 1, 3, 5)
	requireGet(km, 2, key2)
	requireGet(km, 4, key4)
	require.NoError(km.Close())

	// Key 2 and 4 can be retrieved after reopening the file
	km, err = NewKeyManager(fs, "", masterKey)
	require.NoError(err)
	requireGetError(km, 0, 1, 3, 5)
	requireGet(km, 2, key2)
	requireGet(km, 4, key4)
	// Create a new key for file number 2
	// The new key shadows the old key for file number 2. This behavior is needed to securely recover from a crash after persisting the
	// key but before creating the encrypted file. A new key must be used, or otherwise a fork with the same file key could be started.
	key2new := requireCreate(km, 2)
	require.NotEqual(key2, key2new)
	require.NotEqual(key4, key2new)
	requireGetError(km, 0, 1, 3, 5)
	requireGet(km, 2, key2new)
	requireGet(km, 4, key4)
	require.NoError(km.Close())

	// Key 2 and 4 can be retrieved after reopening the file
	km, err = NewKeyManager(fs, "", masterKey)
	require.NoError(err)
	requireGetError(km, 0, 1, 3, 5)
	requireGet(km, 2, key2new)
	requireGet(km, 4, key4)
	require.NoError(km.Close())
}
