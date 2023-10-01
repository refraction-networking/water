//go:build !nogcfix

package water

// GCFIX is a workaround to prevent Go GC from incorrectly garbage
// collecting the cloned `*os.File` pushed to WASM with `PushFile()`.
//
// BUG: There is an undocumented GC issue in Go 1.20 and Go 1.21.
// The first `*os.File` pushed to WASM with `PushFile()` from wasmtime
// will be incorrectly garbage collected by Go GC even if it is still
// accessible from Go.
const GCFIX bool = true
