/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package kvstore

func (f *memFile) WriteApproved(p []byte) error {
	return f.Write(p)
}
