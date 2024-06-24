/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package sstable

import (
	"testing"

	"github.com/edgelesssys/estore/internal/edg"
	"github.com/edgelesssys/estore/objstorage/objstorageprovider"
	"github.com/edgelesssys/estore/vfs"
	"github.com/stretchr/testify/require"
)

func init() {
	edg.TestEnableRandomKey()
}

func (f *memFile) WriteApproved(p []byte) error {
	return f.Write(p)
}

func (f *discardFile) WriteApproved(p []byte) error {
	return f.Write(p)
}

type unencryptedOpt struct{}

func (unencryptedOpt) preApply()                  {}
func (unencryptedOpt) readerApply(reader *Reader) { reader.unencrypted = true }

// encryptSST reads an unencrypted test SST file and returns an encrypted SST file that can be used in tests.
// This copies all the keys and values of the input file, but nothing else, so it may not be sufficient for all tests.
func encryptSST(t *testing.T, testFile ReadableFile) vfs.File {
	require := require.New(t)
	const filename = "foo"
	fs := vfs.NewMem()

	func() {
		reader, err := newReader(testFile, ReaderOptions{}, unencryptedOpt{})
		require.NoError(err)
		defer reader.Close()

		encryptedFile, err := fs.Create(filename)
		require.NoError(err)
		writer := NewWriter(objstorageprovider.NewFileWritable(encryptedFile), WriterOptions{})
		defer writer.Close()

		it, err := reader.NewIter(nil, nil)
		require.NoError(err)
		defer it.Close()

		for {
			key, val := it.Next()
			if key == nil {
				break
			}
			require.NoError(writer.Add(*key, val.ValueOrHandle))
		}
	}()

	encryptedFile, err := fs.Open(filename)
	require.NoError(err)
	return encryptedFile
}
