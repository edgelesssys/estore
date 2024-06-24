/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package sstable

import (
	"context"
	"crypto/cipher"
	"encoding/binary"

	"github.com/cockroachdb/errors"
	"github.com/edgelesssys/estore/internal/base"
	"github.com/edgelesssys/estore/internal/edg"
	"github.com/edgelesssys/estore/objstorage"
)

func (w *Writer) edgEncrypt(bh BlockHandle, block, blockTrailerBuf []byte) []byte {
	buf := append(block, blockTrailerBuf[:blockTrailerLen-edg.GCMTagSize]...)
	return w.aead.Seal(buf[:0], edgGetNonce(bh), buf, nil)
}

func (w *Writer) edgEncryptFooter(encodedFooter []byte, offset uint64) []byte {
	return w.aead.Seal(encodedFooter[:0], edgGetFooterNonce(offset), encodedFooter, nil)
}

func (r *Reader) edgDecrypt(bh BlockHandle, buf []byte) error {
	if r.unencrypted {
		return nil
	}
	if _, err := r.aead.Open(buf[:0], edgGetNonce(bh), buf, nil); err != nil {
		return base.CorruptionErrorf("checksum mismatch: %w", err) // EDG: include "checksum mismatch" in the message to satisfy tests
	}
	return nil
}

func edgGetNonce(bh BlockHandle) []byte {
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce, bh.Offset)
	return nonce
}

func edgGetFooterNonce(offset uint64) []byte {
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce, offset)
	nonce[8] = 1 // use special iv for footer
	return nonce
}

// decryptedFooter is a Readable that can be passed to readFooter.
// We can implement the decryption here and don't need to modify the original readFooter code.
type decryptedFooter struct {
	buf  []byte
	size int64
}

func newDecryptedFooter(readable objstorage.Readable, aead cipher.AEAD) (decryptedFooter, error) {
	f := decryptedFooter{buf: make([]byte, maxFooterLen+edg.GCMTagSize), size: readable.Size()}
	off := f.size - int64(len(f.buf))
	if off < 0 {
		return decryptedFooter{}, errors.New("invalid table (file size is too small)")
	}
	if err := readable.ReadAt(context.Background(), f.buf, off); err != nil {
		return decryptedFooter{}, err
	}
	var err error
	f.buf, err = aead.Open(f.buf[:0], edgGetFooterNonce(uint64(off)), f.buf, nil)
	if err != nil {
		return decryptedFooter{}, err
	}
	return f, nil
}

func (f decryptedFooter) ReadAt(ctx context.Context, p []byte, off int64) error {
	copy(p, f.buf[off-f.size+maxFooterLen:])
	return nil
}

func (f decryptedFooter) Size() int64 {
	return f.size
}

func (decryptedFooter) Close() error {
	panic("not implemented")
}

func (decryptedFooter) NewReadHandle(context.Context) objstorage.ReadHandle {
	panic("not implemented")
}
