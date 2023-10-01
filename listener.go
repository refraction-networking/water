package water

import (
	"fmt"
	"net"
	"sync/atomic"
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

	closed *atomic.Bool
}

// ListenConfig listens on the network address and returns a Listener
// configured with the given Config.
//
// This is the recommended way to create a Listener, unless there are
// other requirements such as supplying a custom net.Listener. In that
// case, a Listener could be created with NewListener() with a Config
// specifying a custom net.Listener.
func ListenConfig(network, address string, config *Config) (net.Listener, error) {
	lis, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	return &Listener{
		Config: config,
		l:      lis,
		closed: new(atomic.Bool),
	}, nil
}

// NewListener creates a Listener with the given Config.
//
// The Config must specify a custom net.Listener, otherwise the
// Accept() method will fail.
func NewListener(config *Config) *Listener {
	return &Listener{
		Config: config,
		closed: new(atomic.Bool),
	}
}

// Accept waits for and returns the next connection after processing
// the data with the WASM module.
//
// The returned net.Conn implements net.Conn and could be seen as
// the inbound connection with a wrapping transport protocol handled
// by the WASM module.
func (l *Listener) Accept() (net.Conn, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("water: listener is closed")
	}

	if l.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}
	if l.l == nil {
		l.Config.mustEmbedListener()
		l.l = l.Config.EmbedListener
	}

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

func (l *Listener) Close() error {
	if l.closed.CompareAndSwap(false, true) {
		return l.l.Close()
	}
	return nil
}

func (l *Listener) Addr() net.Addr {
	return l.l.Addr()
}
