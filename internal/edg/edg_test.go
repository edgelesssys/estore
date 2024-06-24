/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg_test

import (
	"bytes"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/edgelesssys/estore"
	"github.com/edgelesssys/estore/internal/base"
	"github.com/edgelesssys/estore/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReopen creates a db with a key-value pair and reopens it multiple times checking the value. Useful for debugging.
func TestReopen(t *testing.T) {
	require := require.New(t)

	opts := &estore.Options{
		EncryptionKey: testKey(),
		FS:            vfs.NewMem(),
	}
	key := []byte("foo")
	val := []byte("bar")

	db, err := estore.Open("", opts)
	require.NoError(err)
	require.NoError(db.Set(key, val, nil))
	require.NoError(db.Close())

	for i := 0; i < 9; i++ {
		db, err := estore.Open("", opts)
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
	db, err := estore.Open("", &estore.Options{
		EncryptionKey: testKey(),
		FS:            fs,
		Levels:        []estore.LevelOptions{{Compression: estore.NoCompression}},
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

		sdata := string(data)
		assert.NotContains(sdata, "ipsum", filename)
		assert.NotContains(sdata, "adipi", filename)
		assert.NotContains(sdata, "eiusm", filename)
		assert.NotContains(sdata, "dolor", filename)

		// check for expected entropy
		if fileType, _, ok := base.ParseFilename(fs, filename); ok {
			switch fileType {
			case base.FileTypeLock, base.FileTypeCurrent: // these are not encrypted
			case base.FileTypeManifest:
				assert.Greater(entropy(data), 6.6, filename) // manifest is short
			default:
				assert.Greater(entropy(data), 7.5, filename)
			}
		}
	}
}

// TestIntegrity checks that modified bytes in data files cause crypto errors.
func TestIntegrity(t *testing.T) {
	require := require.New(t)

	const db1 = "db1"
	const db2 = "db2"
	fs := vfs.NewMem()
	opts := &estore.Options{
		EncryptionKey: testKey(),
		FS:            fs,
		Logger:        base.NoopLoggerAndTracer{},
	}

	// arange a db
	db, err := estore.Open(db1, opts)
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
			db, openErr := estore.Open(db2, opts)
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
	db, err = estore.Open(db2, opts)
	require.NoError(err)
	require.NoError(db.Close())
}

// TestSSTFromForkIsRejected tests that one can't replace an SST file of a db with one from a forked db.
func TestSSTFromForkIsRejected(t *testing.T) {
	require := require.New(t)

	const dbdir = "db"
	const forkdir = "fork"
	fs := vfs.NewMem()
	opts := &estore.Options{
		EncryptionKey: testKey(),
		FS:            fs,
		Logger:        base.NoopLoggerAndTracer{},
		WALDir:        "wal",
	}

	// create db
	db, err := estore.Open(dbdir, opts)
	require.NoError(err)
	require.NoError(db.Set([]byte("key1"), []byte("val1"), nil))
	require.NoError(db.Flush())
	require.NoError(db.Close())
	require.NoError(fs.RemoveAll(opts.WALDir))

	// create fork
	ok, err := vfs.Clone(fs, fs, dbdir, forkdir)
	require.NoError(err)
	require.True(ok)

	// advance db
	db, err = estore.Open(dbdir, opts)
	require.NoError(err)
	require.NoError(db.Set([]byte("key2"), []byte("val2"), nil))
	require.NoError(db.Flush())
	require.NoError(db.Close())
	require.NoError(fs.RemoveAll(opts.WALDir))

	// advance fork
	db, err = estore.Open(forkdir, opts)
	require.NoError(err)
	require.NoError(db.Set([]byte("key2"), []byte("val2"), nil))
	require.NoError(db.Flush())
	require.NoError(db.Close())
	require.NoError(fs.RemoveAll(opts.WALDir))

	// copy SST from fork to db
	require.NoError(vfs.Copy(fs, "fork/000010.sst", "db/000010.sst"))

	// try to read from db
	db, err = estore.Open(dbdir, opts)
	require.NoError(err)
	_, _, err = db.Get([]byte("key2"))
	require.EqualError(err, "pebble: backing file 000010 error: cipher: message authentication failed")
}

// TestOldDB checks that a DB created with v1.0.0 can be used with the current version.
func TestOldDB(t *testing.T) {
	if os.Getenv("OE_IS_ENCLAVE") == "1" {
		t.Skip("skip for EGo") // because it doesn't have the testdata and there's not much value in runnning this test also in EGo
	}

	require := require.New(t)

	opts := &estore.Options{
		EncryptionKey: testKey(),
		FS:            vfs.NewMem(),
	}

	ok, err := vfs.Clone(vfs.Default, opts.FS, "testdata/db-v1.0.0", "")
	require.NoError(err)
	require.True(ok)

	db, err := estore.Open("", opts)
	require.NoError(err)

	requireGet := func(key, val string) {
		gotVal, closer, err := db.Get([]byte(key))
		require.NoError(err)
		require.EqualValues(val, gotVal)
		require.NoError(closer.Close())
	}

	requireGet("key1", "val1")
	requireGet("key2", "val2")
	require.NoError(db.Set([]byte("key3"), []byte("val3"), nil))
	requireGet("key1", "val1")
	requireGet("key2", "val2")
	requireGet("key3", "val3")
	require.NoError(db.Close())

	db, err = estore.Open("", opts)
	require.NoError(err)
	requireGet("key1", "val1")
	requireGet("key2", "val2")
	requireGet("key3", "val3")
	require.NoError(db.Close())
}

func testKey() []byte {
	return bytes.Repeat([]byte{2}, 16)
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
		"invalid mac",
		"pebble/record: invalid chunk",
	} {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}

	return false
}
