package water

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/gaukas/water/config"
	"github.com/gaukas/water/interfaces"
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
	Config *config.Config
	closed *atomic.Bool
}

// NewListener listens on the network address and returns a Listener
// configured with the given Config.
//
// This is the recommended way to create a Listener, unless there are
// other requirements such as supplying a custom net.Listener. In that
// case, a Listener could be created with WrapListener() with a Config
// specifying a custom net.Listener.
func NewListener(c *config.Config, network, address string) (net.Listener, error) {
	lis, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	config := c.Clone()
	config.NetworkListener = lis

	return &Listener{
		Config: config,
		closed: new(atomic.Bool),
	}, nil
}

// WrapListener creates a Listener with the given Config.
//
// The Config must specify a custom net.Listener, otherwise the
// Accept() method will fail.
func WrapListener(config *config.Config) *Listener {
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
//
// Implements net.Listener.
func (l *Listener) Accept() (net.Conn, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("water: listener is closed")
	}

	if l.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}

	var core interfaces.Core
	var err error
	core, err = Core(l.Config)
	if err != nil {
		return nil, err
	}

	return AcceptVersion(core)
}

// Close closes the listener.
//
// Implements net.Listener.
func (l *Listener) Close() error {
	if l.closed.CompareAndSwap(false, true) {
		return l.Config.NetworkListener.Close()
	}
	return nil
}

// Addr returns the listener's network address.
//
// Implements net.Listener.
func (l *Listener) Addr() net.Addr {
	return l.Config.NetworkListener.Addr()
}
