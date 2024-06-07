// Copyright 2020 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package kvstore_test

import (
	"crypto/rand"
	"fmt"
	"log"

	kvstore "github.com/edgelesssys/ego-kvstore"
	"github.com/edgelesssys/ego-kvstore/vfs"
)

func Example() {
	encryptionKey := make([]byte, 16)
	_, err := rand.Read(encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	db, err := kvstore.Open("", &kvstore.Options{EncryptionKey: encryptionKey, FS: vfs.NewMem()})
	if err != nil {
		log.Fatal(err)
	}
	key := []byte("hello")
	if err := db.Set(key, []byte("world"), kvstore.Sync); err != nil {
		log.Fatal(err)
	}
	value, closer, err := db.Get(key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s %s\n", key, value)
	if err := closer.Close(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	// Output:
	// hello world
}
