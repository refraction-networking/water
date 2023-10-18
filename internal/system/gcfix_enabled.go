//go:build gcbug

package system

// GCBUG is a boolean that indicates whether the garbage collector bug is present.
//
// Garbage Collection bug: Go GC will garbage collect the first file we pushed
// into the WASM module no matter what. Seemingly no one admits this is a bug
// on their side, so we work around it by pushing a dummy file first.
const GCBUG = true
