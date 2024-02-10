package v0

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/gaukas/water"
)

func init() {
	err := water.RegisterWATMListener("_water_v0", NewListenerWithContext)
	if err != nil {
		panic(err)
	}
}

// Listener implements water.Listener utilizing Water WATM API v0.
type Listener struct {
	config *water.Config
	closed *atomic.Bool
	ctx    context.Context

	water.UnimplementedListener // embedded to ensure forward compatibility
}

// NewListener creates a new Listener.
//
// Deprecated: use NewListenerWithContext instead.
func NewListener(c *water.Config) (water.Listener, error) {
	return &Listener{
		config: c.Clone(),
		closed: new(atomic.Bool),
	}, nil
}

// NewListenerWithContext creates a new Listener with the given context.
func NewListenerWithContext(ctx context.Context, c *water.Config) (water.Listener, error) {
	return &Listener{
		config: c.Clone(),
		closed: new(atomic.Bool),
		ctx:    ctx,
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
func (l *Listener) Accept() (net.Conn, error) {
	return l.AcceptWATER()
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

// AcceptWATER waits for and returns the next connection to the listener
// as a water.Conn.
func (l *Listener) AcceptWATER() (water.Conn, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("water: listener is closed")
	}

	if l.config == nil {
		return nil, fmt.Errorf("water: accept with nil config is not allowed")
	}

	var core water.Core
	var err error
	core, err = water.NewCoreWithContext(l.ctx, l.config)
	if err != nil {
		return nil, err
	}

	return accept(core)
}
