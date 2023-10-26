package v0_test

import (
	"os"
	"sync"
)

var (
	plain         []byte
	loadPlainOnce sync.Once
)

func loadPlain() {
	loadPlainOnce.Do(func() {
		var err error
		plain, err = os.ReadFile("../../testdata/v0/plain.wasm")
		if err != nil {
			panic(err)
		}
	})
}
