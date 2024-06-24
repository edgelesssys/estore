package main

import (
	"bytes"

	kvstore "github.com/edgelesssys/ego-kvstore"
)

func main() {
	db, err := kvstore.Open("db-v1.0.0", &kvstore.Options{EncryptionKey: bytes.Repeat([]byte{2}, 16)})
	mustNoErr(err)
	mustNoErr(db.Set([]byte("key1"), []byte("val1"), nil))
	mustNoErr(db.Flush())
	mustNoErr(db.Set([]byte("key2"), []byte("val2"), nil))
	mustNoErr(db.Close())
}

func mustNoErr(err error) {
	if err != nil {
		panic(err)
	}
}
