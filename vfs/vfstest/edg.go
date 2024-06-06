/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package vfstest

func (*discardFile) WriteApproved(p []byte) (int, error) { return len(p), nil }
