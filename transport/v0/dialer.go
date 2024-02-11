package v0

import (
	"context"
	"fmt"

	"github.com/gaukas/water"
)

func init() {
	err := water.RegisterWATMDialer("_water_v0", NewDialerWithContext)
	if err != nil {
		panic(err)
	}
}

// Dialer implements water.Dialer utilizing Water WATM API v0.
type Dialer struct {
	config *water.Config
	ctx    context.Context

	water.UnimplementedDialer // embedded to ensure forward compatibility
}

// NewDialer creates a new [water.Dialer] from the given [water.Config].
//
// Deprecated: use [NewDialerWithContext] instead.
func NewDialer(c *water.Config) (water.Dialer, error) {
	return NewDialerWithContext(context.Background(), c)
}

// NewDialerWithContext creates a new [water.Dialer] from the given [water.Config]
// with the given [context.Context].
//
// The context is used as the default context for call to [Dialer.Dial].
func NewDialerWithContext(ctx context.Context, c *water.Config) (water.Dialer, error) {
	return &Dialer{
		config: c.Clone(),
		ctx:    ctx,
	}, nil
}

// Dial dials the network address using the dialerFunc specified in config.
//
// Implements [water.Dialer].
func (d *Dialer) Dial(network, address string) (conn water.Conn, err error) {
	return d.DialContext(d.ctx, network, address)
}

// DialContext dials the network address using the dialerFunc specified in config.
//
// The context is passed to [water.NewCoreWithContext] to control the lifetime of
// the call to function calls into the WebAssembly module.
// If the context is canceled or reaches its deadline, any current and future
// function call will return with an error.
// Call [water.WazeroRuntimeConfigFactory.SetCloseOnContextDone] with false to
// disable this behavior.
//
// Implements [water.Dialer].
func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn water.Conn, err error) {
	if d.config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}

	ctxReady, dialReady := context.WithCancel(context.Background())
	go func() {
		defer dialReady()
		var core water.Core
		core, err = water.NewCoreWithContext(ctx, d.config)
		if err != nil {
			return
		}

		conn, err = dial(core, network, address)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ctxReady.Done():
		return conn, err
	}
}
