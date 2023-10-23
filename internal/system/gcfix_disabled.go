//go:build !gcbug

package system

// GC_BUG is a boolean flag indicating whether the garbage collector bug is present.
//
// Garbage Collection bug: Go GC will garbage collect the first file we pushed
// into the WASM module no matter what. Seemingly no one admits this is a bug
// on their side, so we work around it by pushing a dummy file first.
//
// First reported in a v0 draft (2023 Sep). It is no longer observed later in the
// finalist v0 standard.
const GC_BUG = false
