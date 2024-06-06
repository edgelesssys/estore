/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg_test

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"

	kvstore "github.com/edgelesssys/ego-kvstore"
	"github.com/edgelesssys/ego-kvstore/internal/base"
	"github.com/edgelesssys/ego-kvstore/internal/edg"
	"github.com/edgelesssys/ego-kvstore/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	edg.TestEnableRandomKey()
}

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

// TestConfidentiality checks that the data files don't contain plaintext.
func TestConfidentiality(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs := vfs.NewMem()
	db, err := kvstore.Open("", &kvstore.Options{
		FS:     fs,
		Levels: []kvstore.LevelOptions{{Compression: kvstore.NoCompression}},
	})
	require.NoError(err)

	// will be in SST
	require.NoError(db.Set([]byte("lorem ipsum dolor sit amet"), []byte("consectetur adipisici elit"), nil))
	require.NoError(db.Flush())

	// will be in WAL
	require.NoError(db.Set([]byte("sed eiusmod tempor incidunt"), []byte("labore et dolore magna aliqua"), nil))
	require.NoError(db.Set([]byte("long"), bytes.Repeat([]byte{0}, 500), nil)) // ensure WAL is long enough so that entropy is meaningful
	require.NoError(db.Close())

	files, err := fs.List("")
	require.NoError(err)

	for _, filename := range files {
		file, err := fs.Open(filename)
		require.NoError(err)
		data, err := io.ReadAll(file)
		require.NoError(err)
		require.NoError(file.Close())

		// check for expected entropy
		if fileType, _, ok := base.ParseFilename(fs, filename); ok {
			switch fileType {
			case base.FileTypeLock, base.FileTypeCurrent: // these are not encrypted
			case base.FileTypeLog, base.FileTypeManifest, base.FileTypeOptions:
				continue // TODO
			default:
				assert.Greater(entropy(data), 7.5, filename)
			}
		}

		sdata := string(data)
		assert.NotContains(sdata, "ipsum", filename)
		assert.NotContains(sdata, "adipi", filename)
		assert.NotContains(sdata, "eiusm", filename)
		assert.NotContains(sdata, "dolor", filename)
	}
}

// TestIntegrity checks that modified bytes in data files cause crypto errors.
func TestIntegrity(t *testing.T) {
	require := require.New(t)

	const db1 = "db1"
	const db2 = "db2"
	fs := vfs.NewMem()
	opts := &kvstore.Options{
		FS:     fs,
		Logger: base.NoopLoggerAndTracer{},
	}

	// arange a db
	db, err := kvstore.Open(db1, opts)
	require.NoError(err)
	require.NoError(db.Set([]byte("key1"), []byte("val1"), nil))
	require.NoError(db.Flush())
	require.NoError(db.Set([]byte("key2"), []byte("val2"), nil))
	require.NoError(db.Close())

	files, err := fs.List(db1)
	require.NoError(err)

	// check that every corrupted byte in every file causes a crypto error
	for _, filename := range files {
		t.Log(filename)

		// read file
		file, err := fs.Open(fs.PathJoin(db1, filename))
		require.NoError(err)
		orgData, err := io.ReadAll(file)
		require.NoError(err)
		require.NoError(file.Close())

		if filename == "CURRENT" {
			continue // CURRENT is not encrypted. Corrupting it doesn't cause crypto errors (but other errors).
		}

		// TODO encrypt these file types
		if fileType, _, ok := base.ParseFilename(fs, filename); ok {
			switch fileType {
			case base.FileTypeLog, base.FileTypeManifest, base.FileTypeOptions:
				continue
			}
		}

		for i := 0; i < len(orgData); i++ {
			// create a clone of the db with one byte in one file corrupted
			data := bytes.Clone(orgData)
			data[i] ^= 1
			ok, err := vfs.Clone(fs, fs, db1, db2)
			require.NoError(err)
			require.True(ok)
			file, err := fs.Create(fs.PathJoin(db2, filename))
			require.NoError(err)
			_, err = file.WriteApproved(data)
			require.NoError(err)
			require.NoError(file.Close())

			// try to open the db and maybe try to get the value (some errors only occur on Get)
			db, openErr := kvstore.Open(db2, opts)
			var getErr error
			if openErr == nil {
				var closer io.Closer
				_, closer, getErr = db.Get([]byte("key1"))
				if getErr == nil {
					require.NoError(closer.Close())
				}
				require.NoError(db.Close())
			}

			require.NoError(fs.RemoveAll(db2))
			require.True(isCryptoError(openErr) || isCryptoError(getErr), "open: %+v\nget: %+v", openErr, getErr)
		}
	}

	// check that opening a copy works in general
	ok, err := vfs.Clone(fs, fs, db1, db2)
	require.NoError(err)
	require.True(ok)
	db, err = kvstore.Open(db2, opts)
	require.NoError(err)
	require.NoError(db.Close())
}

func entropy(data []byte) float64 {
	var freq [256]int
	for _, b := range data {
		freq[b]++
	}

	lenData := float64(len(data))
	var entropy float64
	for _, n := range freq {
		if n > 0 {
			p := float64(n) / lenData
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func isCryptoError(err error) bool {
	if err == nil {
		return false
	}

	for _, s := range []string{
		"cipher: message authentication failed",
		// TODO
	} {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}

	return false
}
