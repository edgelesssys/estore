/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package edg

// Writer is an interface for Write and WriteApproved.
type Writer interface {
	Write([]byte) (int, error)
	WriteApproved([]byte) (int, error)
}

// ApprovedWriter wraps an edg.Writer and provides an io.Writer interface.
// Use this to pass an edg.Writer to functions that expect an io.Writer.
type ApprovedWriter struct {
	Writer interface {
		WriteApproved([]byte) (int, error)
	}
}

func (w *ApprovedWriter) Write(p []byte) (int, error) {
	return w.Writer.WriteApproved(p)
}

// Discard is a Writer on which all Write calls succeed without doing anything.
var Discard Writer = discard{}

type discard struct{}

func (discard) Write(p []byte) (int, error) {
	return len(p), nil
}

func (discard) WriteApproved(p []byte) (int, error) {
	return len(p), nil
}
