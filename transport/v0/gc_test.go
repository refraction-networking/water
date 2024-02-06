package v0

import (
	_ "embed"
)

// var (
// 	//go:embed testdata/plain.wasm
// 	wasmPlain []byte

// 	//go:embed testdata/reverse.wasm
// 	wasmReverse []byte
// )

// This file is specifically designed to test to make sure everything will eventually be
// collected by the garbage collector. This is to ensure that there are no memory leaks
// on our end.

// func TestConn_GC(t *testing.T) {
// 	tcpLis, err := net.Listen("tcp", "localhost:0")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer tcpLis.Close() // skipcq: GO-S2307

// 	// Dial using water
// 	config := &water.Config{
// 		TransportModuleBin: wasmPlain,
// 	}
// 	dialer, err := NewDialer(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// }
