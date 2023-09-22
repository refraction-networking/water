package water

// Unlike RuntimeConn which is a net.Conn that caller
// can Read() from or Write() to, RuntimeListener is
// a net.Listener that caller can use to Listen() on
// a network address and then Accept() connections.
//
// Connection accepted by RuntimeListener is directly
// passed to the WASM module without further interaction.
type RuntimeListener interface {
	// TODO...
}
