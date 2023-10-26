package v0

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/gaukas/water"
)

func init() {
	err := water.RegisterListener("_water_v0", NewListener)
	if err != nil {
		panic(err)
	}
}

// Listener implements water.Listener utilizing Water WATM API v0.
type Listener struct {
	config *water.Config
	closed *atomic.Bool

	water.UnimplementedListener // embedded to ensure forward compatibility
}

// NewListener creates a new Listener.
func NewListener(c *water.Config) (water.Listener, error) {
	return &Listener{
		config: c.Clone(),
		closed: new(atomic.Bool),
	}, nil
}

// Accept waits for and returns the next connection after processing
// the data with the WASM module.
//
// The returned net.Conn implements net.Conn and could be seen as
// the inbound connection with a wrapping transport protocol handled
// by the WASM module.
//
// Implements net.Listener.
func (l *Listener) Accept() (water.Conn, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("water: listener is closed")
	}

	if l.config == nil {
		return nil, fmt.Errorf("water: accept with nil config is not allowed")
	}

	var core water.Core
	var err error
	core, err = water.NewCore(l.config)
	if err != nil {
		return nil, err
	}

	return accept(core)
}

// Close closes the listener.
//
// Implements net.Listener.
func (l *Listener) Close() error {
	if l.closed.CompareAndSwap(false, true) {
		return l.config.NetworkListener.Close()
	}
	return nil
}

// Addr returns the listener's network address.
//
// Implements net.Listener.
func (l *Listener) Addr() net.Addr {
	return l.config.NetworkListener.Addr()
}
