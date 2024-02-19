//go:build wasi || wasip1

package net

import "unsafe"

// Import the host-imported dialer function.
//
//go:wasmimport env host_dial
//go:noescape
func _import_host_dial() (fd int32)

// Import the host-imported acceptor function.
//
//go:wasmimport env host_accept
//go:noescape
func _import_host_accept() (fd int32)

// Import wasi_snapshot_preview1's fd_fdstat_set_flags function
// until tinygo supports it.
//
//go:wasmimport wasi_snapshot_preview1 fd_fdstat_set_flags
//go:noescape
func fd_fdstat_set_flags(fd int32, flags uint32) uint32

// Import wasi_snapshot_preview1's fd_fdstat_set_flags function
// until tinygo supports it.
//
//go:wasmimport wasi_snapshot_preview1 fd_fdstat_get
//go:noescape
func fd_fdstat_get(fd int32, buf unsafe.Pointer) uint32
