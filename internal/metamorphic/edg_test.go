/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package metamorphic

import "github.com/edgelesssys/ego-kvstore/internal/edg"

func init() {
	edg.TestEnableRandomKey()
}
