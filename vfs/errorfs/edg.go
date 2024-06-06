/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package errorfs

func (f *errorFile) WriteApproved(p []byte) (int, error) {
	if err := f.inj.MaybeError(OpFileWrite, f.path); err != nil {
		return 0, err
	}
	return f.file.WriteApproved(p)
}
