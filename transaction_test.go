/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package kvstore

import (
	"testing"

	"github.com/edgelesssys/estore/vfs"
	"github.com/stretchr/testify/require"
)

func newDB(t *testing.T) *DB {
	db, err := Open("", &Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	return db
}

func TestTransactionIsolation(t *testing.T) {
	require := require.New(t)
	db := newDB(t)
	key := []byte("key")
	value1 := []byte("value1")
	value2 := []byte("value2")

	// arrange

	tx := db.NewTransaction(true)
	require.NoError(tx.Set(key, value1, nil))
	require.NoError(tx.Commit())

	// act

	reader1 := db.NewTransaction(false)
	writer := db.NewTransaction(true)
	require.NoError(writer.Set(key, value2, nil))
	reader2 := db.NewTransaction(false)
	require.NoError(writer.Commit())
	reader3 := db.NewTransaction(false)

	// assert

	val, _, err := reader1.Get(key)
	require.NoError(err)
	require.Equal(value1, val)

	val, _, err = reader2.Get(key)
	require.NoError(err)
	require.Equal(value1, val)

	val, _, err = reader3.Get(key)
	require.NoError(err)
	require.Equal(value2, val)
}

func TestTransactionIterator(t *testing.T) {
	require := require.New(t)
	db := newDB(t)
	key := []byte("key")
	value1 := []byte("value1")
	value2 := []byte("value2")

	// arrange

	tx := db.NewTransaction(true)
	require.NoError(tx.Set(key, value1, nil))
	require.NoError(tx.Commit())

	// act

	reader1 := db.NewTransaction(false)
	writer := db.NewTransaction(true)

	writerIt1 := writer.NewIter(nil)
	require.NoError(writer.Set(key, value2, nil))
	writerIt2 := writer.NewIter(nil)
	reader2 := db.NewTransaction(false)

	// assert writer iterators return correct values

	require.True(writerIt1.First())
	require.Equal(key, writerIt1.Key())
	require.Equal(value1, writerIt1.Value())

	require.True(writerIt2.First())
	require.Equal(key, writerIt2.Key())
	require.Equal(value2, writerIt2.Value())

	// commit transaction

	require.NoError(writer.Commit())
	reader3 := db.NewTransaction(false)

	// assert reader iterators return correct values

	it := reader1.NewIter(nil)
	require.True(it.First())
	require.Equal(key, it.Key())
	require.Equal(value1, it.Value())

	it = reader2.NewIter(nil)
	require.True(it.First())
	require.Equal(key, it.Key())
	require.Equal(value1, it.Value())

	it = reader3.NewIter(nil)
	require.True(it.First())
	require.Equal(key, it.Key())
	require.Equal(value2, it.Value())
}
