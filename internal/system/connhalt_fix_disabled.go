//go:build !connhaltbug

package system

// CONN_HALT_BUG is a boolean flag indicating whether the connection halt bug is present.
//
// Connection Halting Bug: for some reason when a TCP Connection is closed by the remote by
// a FIN packet, even tho the Go will be able to detect that and return EOF on the next Read(),
// the async read in WebAssembly may not be able to correctly handle that. In rare cases, the
// non-blocking async read becomes blocking and does not return EOF immediately, and meanwhile
// preventing other async operations from making progress, thus halting the whole system.
//
// This bug is not a runtime-side bug, but it is non-trivial to fix or work around inside the
// WebAssembly environment. Therefore from the runtime-side we work around it by using a
// unix socket to relay between the network socket and the WebAssembly environment. When the
// runtime detects that the network socket is closed, it will close the unix socket, which
// will immediately unblock the async read in WebAssembly.
//
// First reported in a finalist v0 standard (2023 Oct).
const CONN_HALT_BUG = false
