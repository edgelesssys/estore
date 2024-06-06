/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package record

import "encoding/binary"

func edgMakeNonce(iv uint64) []byte {
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce, iv)
	return nonce
}
