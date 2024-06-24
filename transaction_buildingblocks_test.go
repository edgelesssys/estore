/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

// Copyright 2012 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package estore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/cockroachdb/datadriven"
	"github.com/edgelesssys/estore/internal/base"
	"github.com/edgelesssys/estore/vfs"
	"github.com/stretchr/testify/require"
)

func TestTransactionSnapshot(t *testing.T) {
	var d *DB
	var snapshots map[string]*Transaction

	close := func() {
		for _, s := range snapshots {
			s.Close()
		}
		snapshots = nil
		if d != nil {
			require.NoError(t, d.Close())
			d = nil
		}
	}
	defer close()

	randVersion := func() FormatMajorVersion {
		minVersion := formatUnusedPrePebblev1MarkedCompacted
		return FormatMajorVersion(int(minVersion) + rand.Intn(
			int(FormatPrePebblev1MarkedCompacted)-int(minVersion)+1)) // EDG: we don't support SST value blocks
	}
	datadriven.RunTest(t, "testdata/snapshot", func(t *testing.T, td *datadriven.TestData) string {
		switch td.Cmd {
		case "define":
			close()

			var err error
			options := &Options{
				FS:                 vfs.NewMem(),
				FormatMajorVersion: randVersion(),
			}
			if td.HasArg("block-size") {
				var blockSize int
				td.ScanArgs(t, "block-size", &blockSize)
				options.Levels = make([]LevelOptions, 1)
				options.Levels[0].BlockSize = blockSize
				options.Levels[0].IndexBlockSize = blockSize
			}
			d, err = Open("", options)
			if err != nil {
				return err.Error()
			}
			snapshots = make(map[string]*Transaction)

			for _, line := range strings.Split(td.Input, "\n") {
				parts := strings.Fields(line)
				if len(parts) == 0 {
					continue
				}
				var err error
				switch parts[0] {
				case "set":
					if len(parts) != 3 {
						return fmt.Sprintf("%s expects 2 arguments", parts[0])
					}
					err = d.Set([]byte(parts[1]), []byte(parts[2]), nil)
				case "del":
					if len(parts) != 2 {
						return fmt.Sprintf("%s expects 1 argument", parts[0])
					}
					err = d.Delete([]byte(parts[1]), nil)
				case "merge":
					if len(parts) != 3 {
						return fmt.Sprintf("%s expects 2 arguments", parts[0])
					}
					err = d.Merge([]byte(parts[1]), []byte(parts[2]), nil)
				case "snapshot":
					if len(parts) != 2 {
						return fmt.Sprintf("%s expects 1 argument", parts[0])
					}
					snapshots[parts[1]] = d.NewTransaction(false)
				case "compact":
					if len(parts) != 2 {
						return fmt.Sprintf("%s expects 1 argument", parts[0])
					}
					keys := strings.Split(parts[1], "-")
					if len(keys) != 2 {
						return fmt.Sprintf("malformed key range: %s", parts[1])
					}
					err = d.Compact([]byte(keys[0]), []byte(keys[1]), false)
				default:
					return fmt.Sprintf("unknown op: %s", parts[0])
				}
				if err != nil {
					return err.Error()
				}
			}
			return ""

		case "db-state":
			d.mu.Lock()
			s := d.mu.versions.currentVersion().String()
			d.mu.Unlock()
			return s

		case "iter":
			var iter *Iterator
			if len(td.CmdArgs) == 1 {
				if td.CmdArgs[0].Key != "snapshot" {
					return fmt.Sprintf("unknown argument: %s", td.CmdArgs[0])
				}
				if len(td.CmdArgs[0].Vals) != 1 {
					return fmt.Sprintf("%s expects 1 value: %s", td.CmdArgs[0].Key, td.CmdArgs[0])
				}
				name := td.CmdArgs[0].Vals[0]
				snapshot := snapshots[name]
				if snapshot == nil {
					return fmt.Sprintf("unable to find snapshot \"%s\"", name)
				}
				iter = snapshot.NewIter(nil)
			} else {
				iter, _ = d.NewIter(nil)
			}
			defer iter.Close()

			var b bytes.Buffer
			for _, line := range strings.Split(td.Input, "\n") {
				parts := strings.Fields(line)
				if len(parts) == 0 {
					continue
				}
				switch parts[0] {
				case "first":
					iter.First()
				case "last":
					iter.Last()
				case "seek-ge":
					if len(parts) != 2 {
						return "seek-ge <key>\n"
					}
					iter.SeekGE([]byte(strings.TrimSpace(parts[1])))
				case "seek-lt":
					if len(parts) != 2 {
						return "seek-lt <key>\n"
					}
					iter.SeekLT([]byte(strings.TrimSpace(parts[1])))
				case "next":
					iter.Next()
				case "prev":
					iter.Prev()
				default:
					return fmt.Sprintf("unknown op: %s", parts[0])
				}
				if iter.Valid() {
					fmt.Fprintf(&b, "%s:%s\n", iter.Key(), iter.Value())
				} else if err := iter.Error(); err != nil {
					fmt.Fprintf(&b, "err=%v\n", err)
				} else {
					fmt.Fprintf(&b, ".\n")
				}
			}
			return b.String()

		default:
			return fmt.Sprintf("unknown command: %s", td.Cmd)
		}
	})
}

func TestTransactionBatch(t *testing.T) {
	type testCase struct {
		kind       InternalKeyKind
		key, value string
		valueInt   uint32
	}

	verifyTestCases := func(b *Transaction, testCases []testCase, indexedPointKindsOnly bool) {
		r := b.Reader()

		for _, tc := range testCases {
			if indexedPointKindsOnly && (tc.kind == InternalKeyKindLogData || tc.kind == InternalKeyKindIngestSST ||
				tc.kind == InternalKeyKindRangeKeyUnset || tc.kind == InternalKeyKindRangeKeySet ||
				tc.kind == InternalKeyKindRangeKeyDelete || tc.kind == InternalKeyKindRangeDelete) {
				continue
			}
			kind, k, v, ok, err := r.Next()
			if !ok {
				if err != nil {
					t.Fatal(err)
				}
				t.Fatalf("next returned !ok: test case = %v", tc)
			}
			key, value := string(k), string(v)
			if kind != tc.kind || key != tc.key || value != tc.value {
				t.Errorf("got (%d, %q, %q), want (%d, %q, %q)",
					kind, key, value, tc.kind, tc.key, tc.value)
			}
		}
		if len(r) != 0 {
			t.Errorf("reader was not exhausted: remaining bytes = %q", r)
		}
	}

	encodeFileNum := func(n base.FileNum) string {
		return string(binary.AppendUvarint(nil, uint64(n)))
	}
	decodeFileNum := func(d []byte) base.FileNum {
		val, n := binary.Uvarint(d)
		if n <= 0 {
			t.Fatalf("invalid filenum encoding")
		}
		return base.FileNum(val)
	}

	// RangeKeySet and RangeKeyUnset are untested here because they don't expose
	// deferred variants. This is a consequence of these keys' more complex
	// value encodings.
	testCases := []testCase{
		{InternalKeyKindIngestSST, encodeFileNum(1), "", 0},
		{InternalKeyKindSet, "roses", "red", 0},
		{InternalKeyKindSet, "violets", "blue", 0},
		{InternalKeyKindDelete, "roses", "", 0},
		{InternalKeyKindSingleDelete, "roses", "", 0},
		{InternalKeyKindSet, "", "", 0},
		{InternalKeyKindSet, "", "non-empty", 0},
		{InternalKeyKindDelete, "", "", 0},
		{InternalKeyKindSingleDelete, "", "", 0},
		{InternalKeyKindSet, "grass", "green", 0},
		{InternalKeyKindSet, "grass", "greener", 0},
		{InternalKeyKindSet, "eleventy", strings.Repeat("!!11!", 100), 0},
		{InternalKeyKindDelete, "nosuchkey", "", 0},
		{InternalKeyKindDeleteSized, "eleventy", string(binary.AppendUvarint([]byte(nil), 508)), 500},
		{InternalKeyKindSingleDelete, "nosuchkey", "", 0},
		{InternalKeyKindSet, "binarydata", "\x00", 0},
		{InternalKeyKindSet, "binarydata", "\xff", 0},
		{InternalKeyKindMerge, "merge", "mergedata", 0},
		{InternalKeyKindMerge, "merge", "", 0},
		{InternalKeyKindMerge, "", "", 0},
		{InternalKeyKindRangeDelete, "a", "b", 0},
		{InternalKeyKindRangeDelete, "", "", 0},
		{InternalKeyKindLogData, "logdata", "", 0},
		{InternalKeyKindLogData, "", "", 0},
		{InternalKeyKindRangeKeyDelete, "grass", "green", 0},
		{InternalKeyKindRangeKeyDelete, "", "", 0},
		{InternalKeyKindDeleteSized, "nosuchkey", string(binary.AppendUvarint([]byte(nil), 11)), 2},
	}
	db, err := Open("", &Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	b := db.NewTransaction(true)
	for _, tc := range testCases {
		switch tc.kind {
		case InternalKeyKindSet:
			_ = b.Set([]byte(tc.key), []byte(tc.value), nil)
		case InternalKeyKindMerge:
			_ = b.Merge([]byte(tc.key), []byte(tc.value), nil)
		case InternalKeyKindDelete:
			_ = b.Delete([]byte(tc.key), nil)
		case InternalKeyKindDeleteSized:
			_ = b.DeleteSized([]byte(tc.key), tc.valueInt, nil)
		case InternalKeyKindSingleDelete:
			_ = b.SingleDelete([]byte(tc.key), nil)
		case InternalKeyKindRangeDelete:
			_ = b.DeleteRange([]byte(tc.key), []byte(tc.value), nil)
		case InternalKeyKindLogData:
			_ = b.LogData([]byte(tc.key), nil)
		case InternalKeyKindRangeKeyDelete:
			_ = b.RangeKeyDelete([]byte(tc.key), []byte(tc.value), nil)
		case InternalKeyKindIngestSST:
			b.ingestSST(decodeFileNum([]byte(tc.key)))
		}
	}
	verifyTestCases(b, testCases, false /* indexedKindsOnly */)

	b.Reset()
	// Run the same operations, this time using the Deferred variants of each
	// operation (eg. SetDeferred).
	for _, tc := range testCases {
		key := []byte(tc.key)
		value := []byte(tc.value)
		switch tc.kind {
		case InternalKeyKindSet:
			d := b.SetDeferred(len(key), len(value))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		case InternalKeyKindMerge:
			d := b.MergeDeferred(len(key), len(value))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		case InternalKeyKindDelete:
			d := b.DeleteDeferred(len(key))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		case InternalKeyKindDeleteSized:
			d := b.DeleteSizedDeferred(len(tc.key), tc.valueInt)
			copy(d.Key, key)
			d.Finish()
		case InternalKeyKindSingleDelete:
			d := b.SingleDeleteDeferred(len(key))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		case InternalKeyKindRangeDelete:
			d := b.DeleteRangeDeferred(len(key), len(value))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		case InternalKeyKindLogData:
			_ = b.LogData([]byte(tc.key), nil)
		case InternalKeyKindIngestSST:
			b.ingestSST(decodeFileNum([]byte(tc.key)))
		case InternalKeyKindRangeKeyDelete:
			d := b.RangeKeyDeleteDeferred(len(key), len(value))
			copy(d.Key, key)
			copy(d.Value, value)
			d.Finish()
		}
	}
	verifyTestCases(b, testCases, false /* indexedKindsOnly */)

	b.Reset()
	// Run the same operations, this time using AddInternalKey instead of the
	// Kind-specific methods.
	for _, tc := range testCases {
		if tc.kind == InternalKeyKindLogData || tc.kind == InternalKeyKindIngestSST ||
			tc.kind == InternalKeyKindRangeKeyUnset || tc.kind == InternalKeyKindRangeKeySet ||
			tc.kind == InternalKeyKindRangeKeyDelete || tc.kind == InternalKeyKindRangeDelete {
			continue
		}
		key := []byte(tc.key)
		value := []byte(tc.value)
		b.AddInternalKey(&InternalKey{UserKey: key, Trailer: base.MakeTrailer(0, tc.kind)}, value, nil)
	}
	verifyTestCases(b, testCases, true /* indexedKindsOnly */)
}
