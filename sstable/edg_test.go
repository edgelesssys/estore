/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package sstable

func (f *memFile) WriteApproved(p []byte) error {
	return f.Write(p)
}

func (f *discardFile) WriteApproved(p []byte) error {
	return f.Write(p)
}
