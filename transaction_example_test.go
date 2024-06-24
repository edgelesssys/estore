/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package estore_test

import (
	"crypto/rand"
	"fmt"
	"log"

	"github.com/edgelesssys/estore"
	"github.com/edgelesssys/estore/vfs"
)

func ExampleTransaction() {
	encryptionKey := make([]byte, 16)
	_, err := rand.Read(encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	db, err := estore.Open("", &estore.Options{EncryptionKey: encryptionKey, FS: vfs.NewMem()})
	if err != nil {
		panic(err)
	}

	// Write key-value pairs in a write transaction.

	tx := db.NewTransaction(true)
	defer tx.Close()

	if err := tx.Set([]byte("key1"), []byte("value1"), nil); err != nil {
		panic(err)
	}
	if err := tx.Set([]byte("key2"), []byte("value2"), nil); err != nil {
		panic(err)
	}

	if err := tx.Commit(); err != nil {
		panic(err)
	}

	// Read the values back.

	tx = db.NewTransaction(false)
	defer tx.Close()

	val, closer, err := tx.Get([]byte("key1"))
	if err != nil {
		panic(err)
	}
	defer closer.Close()
	fmt.Println(string(val))

	val, closer, err = tx.Get([]byte("key2"))
	if err != nil {
		panic(err)
	}
	defer closer.Close()
	fmt.Println(string(val))

	// Output:
	// value1
	// value2
}
