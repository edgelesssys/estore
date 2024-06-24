/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package estore

import (
	"context"
	"io"
)

// NewTransaction starts a new transaction.
//
// Read transactions can be run concurrently, but only one write transaction can be run at a time.
// If additional write transactions are started, the calls to this function will block until the current write transaction is closed.
func (d *DB) NewTransaction(writable bool) *Transaction {
	if !writable {
		return &Transaction{snap: d.NewSnapshot()}
	}
	d.txLock.Lock()
	return &Transaction{Batch: d.NewIndexedBatch(), snap: d.NewSnapshot()}
}

// Transaction is a database transaction.
//
// Transactions must be closed by calling Close or Commit when they are no longer needed.
// You must not perform non-transctional write operations if a write transaction is active.
type Transaction struct {
	*Batch
	snap *Snapshot
}

// Commit commits and closes the transaction.
func (t *Transaction) Commit() error {
	defer t.Close()
	return t.Batch.Commit(nil)
}

// Close closes the transaction without committing it.
//
// It is valid but not required to call Close after Commit.
func (t *Transaction) Close() {
	if t.snap == nil {
		return
	}
	if t.Batch != nil {
		t.db.txLock.Unlock()
		t.Batch.Close()
	}
	t.snap.Close()
	t.snap = nil
}

// Get gets the value for the given key. It returns ErrNotFound if the key is
// not found.
//
// The caller should not modify the contents of the returned slice, but it is
// safe to modify the contents of the argument after Get returns. The returned
// slice will remain valid until the returned Closer is closed. On success, the
// caller MUST call closer.Close() or a memory leak will occur.
func (t *Transaction) Get(key []byte) ([]byte, io.Closer, error) {
	if t.Batch == nil {
		return t.snap.Get(key)
	}
	return t.db.getInternal(key, t.Batch, t.snap)
}

// NewIter returns an iterator that is unpositioned (Iterator.Valid() will
// return false). The iterator can be positioned via a call to SeekGE,
// SeekLT, First or Last.
func (t *Transaction) NewIter(o *IterOptions) *Iterator {
	return t.NewIterWithContext(context.Background(), o)
}

// NewIterWithContext is like NewIter, and additionally accepts a context for
// tracing.
func (t *Transaction) NewIterWithContext(ctx context.Context, o *IterOptions) *Iterator {
	if t.Batch == nil {
		it, _ := t.snap.NewIterWithContext(ctx, o)
		return it
	}
	return t.Batch.NewIterWithContext(ctx, o)
}
