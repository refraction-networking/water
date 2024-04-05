package water_test

import (
	_ "embed"
)

var (
	//go:embed transport/v1/testdata/plain.wasm
	wasmPlain []byte //nolint:unused

	//go:embed transport/v1/testdata/reverse.wasm
	wasmReverse []byte
)
