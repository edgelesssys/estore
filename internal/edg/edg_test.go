/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg_test

import (
	"testing"

	kvstore "github.com/edgelesssys/ego-kvstore"
	"github.com/edgelesssys/ego-kvstore/vfs"
	"github.com/stretchr/testify/require"
)

// TestReopen creates a db with a key-value pair and reopens it multiple times checking the value. Useful for debugging.
func TestReopen(t *testing.T) {
	require := require.New(t)

	opts := &kvstore.Options{
		FS: vfs.NewMem(),
	}
	key := []byte("foo")
	val := []byte("bar")

	db, err := kvstore.Open("", opts)
	require.NoError(err)
	require.NoError(db.Set(key, val, nil))
	require.NoError(db.Close())

	for i := 0; i < 9; i++ {
		db, err := kvstore.Open("", opts)
		require.NoError(err)
		gotVal, closer, err := db.Get(key)
		require.NoError(err)
		require.Equal(val, gotVal)
		require.NoError(closer.Close())
		require.NoError(db.Close())
	}
}
