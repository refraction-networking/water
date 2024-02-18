//go:build wasip1 || wasi

package v0

import "unsafe"

//go:wasmimport env host_defer
//go:noescape
func _import_host_defer()

//go:wasmimport env pull_config
//go:noescape
func _import_pull_config() (fd int32)

//go:wasmimport wasi_snapshot_preview1 poll_oneoff
//go:noescape
func poll_oneoff(in, out unsafe.Pointer, nsubscriptions size, nevents unsafe.Pointer) errno
