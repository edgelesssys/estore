# EStore

EStore is a key-value store with authenticated encryption for data at rest.
It's based on [Pebble](https://github.com/cockroachdb/pebble), the key-value store used in CockroachDB.
EStore provides confidentiality *and* integrity for the database state as a whole.
We call this "snapshot integrity."
In contrast, other database encryption schemes typically only provide integrity at the record or file level.
As a result, in those cases, attackers can modify parts of the database state unnoticed.

With these properties, EStore is particularly well suited for use with [EGo](https://github.com/edgelesssys/ego) to build [confidential-computing apps](https://www.edgeless.systems/confidential-computing).
However, you can use EStore in any Go application to store sensitive information in a structured way.

## Example

```go
package main

import (
	"crypto/rand"
	"fmt"
	"log"

	"github.com/edgelesssys/estore"
)

func main() {
	// Generate an encryption key
	encryptionKey := make([]byte, 16)
	_, err := rand.Read(encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create an encrypted store
	opts := &estore.Options{
		EncryptionKey: encryptionKey,
	}
	db, err := estore.Open("demo", opts)
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

EStore is licensed under [AGPL-3.0](LICENSE).
It uses code licensed under a [BSD-style license](LICENSE.pebble).

You can also get a [commercial license and enterprise support](https://www.edgeless.systems/enterprise-support).
