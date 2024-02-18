//go:build wasip1 || wasi

package addon

// Imports a few functions from host

// Import from host.
//
//go:wasmimport env send_nuke
func sendNuke(count int32)

// Import from host.
//
//go:wasmimport env cancel_nuke
func cancelNuke() (errno int32)
