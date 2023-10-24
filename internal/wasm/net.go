package wasm

import "github.com/bytecodealliance/wasmtime-go/v13"

// WASMTIMEStoreIndependentFunction is a function that takes a store at
// runtime to work with.
type WASMTIMEStoreIndependentFunction = func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap)
