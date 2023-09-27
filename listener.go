package water

import (
	"fmt"
	"net"
)

// Listener listens on a local network address and upon caller
// calling Accept(), it accepts an incoming connection and
// passes it to the WASM module, which returns a net.Conn to
// caller.
//
// The structure of a Listener is as follows:
//
//	            +---------------+ accept +---------------+ accept
//	       ---->|               |------->|     Decode    |------->
//	Source      |  net.Listener |        |  WASM Runtime |         Caller
//	       <----|               |<-------| Decode/Encode |<-------
//	            +---------------+        +---------------+
//	                     \                      /
//	                      \------Listener------/
//
// As shown above, a Listener consists of a net.Listener to accept
// incoming connections and a WASM runtime to handle the incoming
// connections from an external source. The WASM runtime will return
// a net.Conn that caller can Read() from or Write() to.
//
// The WASM module used by a Listener must implement a WASMListener.
type Listener struct {
	Config *Config
	l      net.Listener
}

func (l *Listener) Accept() (RuntimeConn, error) {
	if l.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}
	if l.l == nil {
		l.Config.mustEmbedListener()
	}
	l.l = l.Config.EmbedListener

	l.Config.mustSetWABin()

	var core *runtimeCore
	var err error
	core, err = Core(l.Config)
	if err != nil {
		return nil, err
	}

	// link listener funcs
	wasiListener := MakeWASIListener(l.l, l.Config.WASIApplicationProtocolWrapper)
	if err = core.LinkNetworkInterface(nil, wasiListener); err != nil {
		return nil, err
	}

	err = core.Initialize()
	if err != nil {
		return nil, err
	}

	return core.InboundRuntimeConn()
}
