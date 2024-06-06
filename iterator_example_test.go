// Copyright 2021 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package kvstore_test

import (
	"fmt"
	"log"

	"github.com/edgelesssys/ego-kvstore"
	"github.com/edgelesssys/ego-kvstore/vfs"
)

func ExampleIterator() {
	db, err := kvstore.Open("", &kvstore.Options{FS: vfs.NewMem()})
	if err != nil {
		log.Fatal(err)
	}

	keys := []string{"hello", "world", "hello world"}
	for _, key := range keys {
		if err := db.Set([]byte(key), nil, kvstore.Sync); err != nil {
			log.Fatal(err)
		}
	}

	iter, _ := db.NewIter(nil)
	for iter.First(); iter.Valid(); iter.Next() {
		fmt.Printf("%s\n", iter.Key())
	}
	if err := iter.Close(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	// Output:
	// hello
	// hello world
	// world
}

func ExampleIterator_prefixIteration() {
	db, err := kvstore.Open("", &kvstore.Options{FS: vfs.NewMem()})
	if err != nil {
		log.Fatal(err)
	}

	keyUpperBound := func(b []byte) []byte {
		end := make([]byte, len(b))
		copy(end, b)
		for i := len(end) - 1; i >= 0; i-- {
			end[i] = end[i] + 1
			if end[i] != 0 {
				return end[:i+1]
			}
		}
		return nil // no upper-bound
	}

	prefixIterOptions := func(prefix []byte) *kvstore.IterOptions {
		return &kvstore.IterOptions{
			LowerBound: prefix,
			UpperBound: keyUpperBound(prefix),
		}
	}

	keys := []string{"hello", "world", "hello world"}
	for _, key := range keys {
		if err := db.Set([]byte(key), nil, kvstore.Sync); err != nil {
			log.Fatal(err)
		}
	}

	iter, _ := db.NewIter(prefixIterOptions([]byte("hello")))
	for iter.First(); iter.Valid(); iter.Next() {
		fmt.Printf("%s\n", iter.Key())
	}
	if err := iter.Close(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	// Output:
	// hello
	// hello world
}

func ExampleIterator_SeekGE() {
	db, err := kvstore.Open("", &kvstore.Options{FS: vfs.NewMem()})
	if err != nil {
		log.Fatal(err)
	}

	keys := []string{"hello", "world", "hello world"}
	for _, key := range keys {
		if err := db.Set([]byte(key), nil, kvstore.Sync); err != nil {
			log.Fatal(err)
		}
	}

	iter, _ := db.NewIter(nil)
	if iter.SeekGE([]byte("a")); iter.Valid() {
		fmt.Printf("%s\n", iter.Key())
	}
	if iter.SeekGE([]byte("hello w")); iter.Valid() {
		fmt.Printf("%s\n", iter.Key())
	}
	if iter.SeekGE([]byte("w")); iter.Valid() {
		fmt.Printf("%s\n", iter.Key())
	}
	if err := iter.Close(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	// Output:
	// hello
	// hello world
	// world
}
