package water

// Relay listens on a local network address and handles requests
// on incoming connections by passing the incoming connection to
// the WASM module and dial corresponding outbound connections
// to the pre-defined destination address, which can either be a
// remote TCP/UDP address or a unix socket.
//
// The structure of a Server is as follows:
//
//	        accept +---------------+      +---------------+ dial
//	       ------->|               |----->|    Decode     |----->
//	Source         |  net.Listener |      | WASM Runtime  |       Remote
//	       <-------|               |<-----| Decode/Encode |<-----
//	               +---------------+      +---------------+
//	                        \                    /
//	                         \------Relay-------/
//
// As shown above, a Server consists of a net.Listener to accept
// incoming connections and a WASM runtime to handle the incoming
// connections from an external source. The WASM runtime will dial
// the corresponding outbound connections to a pre-defined
// destination address. It requires no further caller interaction
// once it is started.
//
// The WASM module used by a Server must implement a WASMDialer.
type Relay struct{}

/// TODO: Server will NOT be implemented without WASI multi-threading
/// support or blocking loop support.
