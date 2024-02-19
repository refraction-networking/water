//go:build wasip1 || wasi

package main

// Import from host.
//
//go:wasmimport env hello
func hello()

// main is required for the `wasi` target, even if it isn't used.
func main() {
	hello()
}
