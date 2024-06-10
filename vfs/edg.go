/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package vfs

import "github.com/cockroachdb/errors"

func (f *enospcFile) WriteApproved(p []byte) (n int, err error) {
	gen := f.fs.waitUntilReady()

	n, err = f.inner.Write(p)

	if err != nil && isENOSPC(err) {
		f.fs.handleENOSPC(gen)
		var n2 int
		n2, err = f.inner.WriteApproved(p[n:])
		n += n2
	}
	return n, err
}

func (f *syncingFile) WriteApproved(p []byte) (n int, err error) {
	_ = f.preallocate(f.offset.Load())

	n, err = f.File.WriteApproved(p)
	if err != nil {
		return n, errors.WithStack(err)
	}
	// The offset is updated atomically so that it can be accessed safely from
	// Sync.
	f.offset.Add(int64(n))
	if err := f.maybeSync(); err != nil {
		return 0, err
	}
	return n, nil
}

func (d *diskHealthCheckingFile) WriteApproved(p []byte) (n int, err error) {
	d.timeDiskOp(OpTypeWrite, int64(len(p)), func() {
		n, err = d.file.WriteApproved(p)
	})
	return n, err
}
