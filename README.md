# ego-kvstore

ego-kvstore is a key-value store with authenticated encryption for data at rest, based on [Pebble](https://github.com/cockroachdb/pebble).
It provides confidentiality and integrity of the whole database state.
In contrast, other databases often only provide confidentiality of the data and sometimes integrity of individual records or file blocks.

With this encryption, ego-kvstore is especially suitable to be used with [EGo](https://github.com/edgelesssys/ego) to build [confidential-computing](https://www.edgeless.systems/confidential-computing) apps.
But you can also use it for any Go application that needs to store sensitive data securely.

## Example

```go
package main

import (
	"crypto/rand"
	"fmt"
	"log"

	kvstore "github.com/edgelesssys/ego-kvstore"
)

func main() {
	// Generate an encryption key
	encryptionKey := make([]byte, 16)
	_, err := rand.Read(encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create an encrypted store
	opts := &kvstore.Options{
		EncryptionKey: encryptionKey,
	}
	db, err := kvstore.Open("demo", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Set a key-value pair
	key := []byte("hello")
	if err := db.Set(key, []byte("world"), nil); err != nil {
		log.Fatal(err)
	}

	// Get the value of the key
	value, closer, err := db.Get(key)
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()
	fmt.Printf("%s %s\n", key, value)
}
```

## License

ego-kvstore is licensed under [AGPL-3.0](LICENSE).
It uses code licensed under a [BSD-style license](LICENSE.pebble).

You can also get a [commercial license and enterprise support](https://www.edgeless.systems/enterprise-support).
