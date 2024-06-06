/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package vfs

import "time"

func (m mockFile) WriteApproved(p []byte) (int, error) {
	time.Sleep(m.syncAndWriteDuration)
	return len(p), nil
}
