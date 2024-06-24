/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package record

import (
	"io"

	"github.com/edgelesssys/estore/internal/edg"
)

func init() {
	edg.TestEnableRandomKey()
}

func (f *syncFile) WriteApproved(buf []byte) (int, error) {
	return f.Write(buf)
}

func (f *syncFileWithWait) WriteApproved(buf []byte) (int, error) {
	return f.Write(buf)
}

type approvedWriter struct {
	io.Writer
}

func (w *approvedWriter) WriteApproved(p []byte) (int, error) {
	return w.Write(p)
}
