/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package kvstore

import (
	"github.com/edgelesssys/ego-kvstore/internal/edg"
	"github.com/edgelesssys/ego-kvstore/vfs/errorfs"
)

func init() {
	edg.TestEnableRandomKey()
}

func (f *memFile) WriteApproved(p []byte) error {
	return f.Write(p)
}

type edgInjectIndexButNotOnRemove struct {
	*errorfs.InjectIndex
}

func (i edgInjectIndexButNotOnRemove) MaybeError(op errorfs.Op, path string) error {
	if op == errorfs.OpRemove {
		return nil
	}
	return i.InjectIndex.MaybeError(op, path)
}
