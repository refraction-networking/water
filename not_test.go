package water_test

import (
	_ "embed"
)

var (
	//go:embed transport/v0/testdata/plain.wasm
	wasmPlain []byte //nolint:unused

	//go:embed transport/v0/testdata/reverse.wasm
	wasmReverse []byte
)
