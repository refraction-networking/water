package water

import (
	"context"
	"fmt"
)

// Dialer dials the given network address upon caller calling
// Dial() and returns a net.Conn which is connected to the
// WASM module.
//
// The structure of a Dialer is as follows:
//
//		        dial +----------------+ dial
//		       ----->|     Decode     |------>
//		Caller       |  WASM Runtime  |        Destination
//		       <-----| Decode/Encode  |<------
//		             +----------------+
//	                    Dialer
type Dialer struct {
	// Config is the configuration for the core.
	Config *Config
}

func (d *Dialer) Dial(network, address string) (RuntimeConn, error) {
	return d.DialContext(context.Background(), network, address)
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (rConn RuntimeConn, err error) {
	if d.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}
	d.Config.defaultNetworkDialerIfNotSet()
	d.Config.requireWABin()

	ctxReady, dialReady := context.WithCancel(context.Background())
	go func() {
		defer dialReady()
		var core *runtimeCore
		core, err = Core(d.Config)
		if err != nil {
			return
		}

		// link dialer funcs
		if err = core.LinkNetworkDialer(d.Config.NetworkDialer, network, address); err != nil {
			return
		}

		// link defer funcs
		if err = core.LinkDefer(); err != nil {
			return
		}

		err = core.Initialize()
		if err != nil {
			return
		}

		rConn, err = core.OutboundRuntimeConn() // will return versioned RuntimeConn
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ctxReady.Done():
		return rConn, err
	}
}
