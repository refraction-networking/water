package water

import (
	"context"
	"fmt"

	"github.com/gaukas/water/config"
	"github.com/gaukas/water/interfaces"
)

// Dialer dials the given network address upon caller calling
// Dial() and returns a net.Conn which is connected to the
// WASM module.
//
// The structure of a Dialer is as follows:
//
//	        dial +----------------+ dial
//	       ----->|     Decode     |------>
//	Caller       |  WASM Runtime  |        Remote
//	       <-----| Decode/Encode  |<------
//	             +----------------+
//	                   Dialer
type Dialer struct {
	// Config is the configuration for the core.
	Config *config.Config
}

func NewDialer(c *config.Config) *Dialer {
	return &Dialer{
		Config: c.Clone(),
	}
}

// Dialer dials the given network address using the specified dialer
// in the config. The returned RuntimeConn implements net.Conn and
// could be seen as the outbound connection with a wrapping transport
// protocol handled by the WASM module.
//
// Internally, DialContext() is called with a background context.
func (d *Dialer) Dial(network, address string) (interfaces.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext dials the given network address using the specified dialer
// in the config. The returned RuntimeConn implements net.Conn and
// could be seen as the outbound connection with a wrapping transport
// protocol handled by the WASM module.
//
// If the context expires before the connection is complete, an error is
// returned.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn interfaces.Conn, err error) {
	if d.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}

	ctxReady, dialReady := context.WithCancel(context.Background())
	go func() {
		defer dialReady()
		var core interfaces.Core
		core, err = Core(d.Config)
		if err != nil {
			return
		}

		conn, err = DialVersion(core, network, address)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ctxReady.Done():
		return conn, err
	}
}
