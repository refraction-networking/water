package wasm

import "github.com/bytecodealliance/wasmtime-go/v13"

type WASMTIMEStoreIndependentFunction = func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap)
