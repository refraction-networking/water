package v0

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/gaukas/water"
)

func init() {
	err := water.RegisterWATMRelay("_water_v0", NewRelayWithContext)
	if err != nil {
		panic(err)
	}
}

// Relay implements water.Relay utilizing Water WATM API v0.
type Relay struct {
	config  *water.Config
	ctx     context.Context
	running *atomic.Bool

	dialNetwork, dialAddress string

	water.UnimplementedRelay // embedded to ensure forward compatibility
}

// NewRelay creates a new [water.Relay] from the given [water.Config] without starting
// it. To start the relay, call [Relay.RelayTo] or [Relay.ListenAndRelayTo].
//
// Deprecated: use [NewRelayWithContext] instead.
func NewRelay(c *water.Config) (water.Relay, error) {
	return NewRelayWithContext(context.Background(), c)
}

// NewRelayWithContext creates a new [water.Relay] from the [water.Config] with the given
// [context.Context] without starting it. To start the relay, call [Relay.RelayTo]
// or [Relay.ListenAndRelayTo].
//
// The context is passed to [water.NewCoreWithContext] to control the lifetime of
// the call to function calls into the WebAssembly module.
// If the context is canceled or reaches its deadline, any current and future
// function call will return with an error.
// Call [water.WazeroRuntimeConfigFactory.SetCloseOnContextDone] with false to
// disable this behavior.
func NewRelayWithContext(ctx context.Context, c *water.Config) (water.Relay, error) {
	return &Relay{
		config:  c.Clone(),
		ctx:     ctx,
		running: new(atomic.Bool),
	}, nil
}

// RelayTo implements [water.Relay].
func (r *Relay) RelayTo(network, address string) error {
	if !r.running.CompareAndSwap(false, true) {
		return water.ErrRelayAlreadyStarted
	}

	if r.config == nil {
		return fmt.Errorf("water: relaying with nil config is not allowed")
	}

	r.dialNetwork = network
	r.dialAddress = address

	var core water.Core
	var err error
	for r.running.Load() {
		core, err = water.NewCoreWithContext(r.ctx, r.config)
		if err != nil {
			return err
		}

		_, err = relay(core, network, address)
		if err != nil {
			if r.running.Load() { // errored before closing
				return err
			}
			break
		}
	}

	return nil
}

// ListenAndRelayTo implements [water.Relay].
func (r *Relay) ListenAndRelayTo(lnetwork, laddress, rnetwork, raddress string) error {
	if !r.running.CompareAndSwap(false, true) {
		return water.ErrRelayAlreadyStarted
	}
	defer r.running.CompareAndSwap(true, false)

	lis, err := net.Listen(lnetwork, laddress)
	if err != nil {
		return err
	}

	config := r.config.Clone()
	config.NetworkListener = lis
	r.config = config

	if r.config == nil {
		return fmt.Errorf("water: relaying with nil config is not allowed")
	}

	r.dialNetwork = rnetwork
	r.dialAddress = raddress

	var core water.Core
	for r.running.Load() {
		core, err = water.NewCoreWithContext(r.ctx, r.config)
		if err != nil {
			return err
		}

		_, err = relay(core, rnetwork, raddress)
		if err != nil {
			if r.running.Load() { // errored before closing
				return err
			}
			break
		}
	}

	return nil
}

// Close implements [water.Relay].
func (r *Relay) Close() error {
	if !r.running.CompareAndSwap(true, false) {
		return nil
	}

	if r.config != nil {
		return r.config.NetworkListener.Close()
	}

	return fmt.Errorf("water: relay is not configured")
}

// Addr implements [water.Relay].
func (r *Relay) Addr() net.Addr {
	if r.config == nil {
		return nil
	}

	return r.config.NetworkListener.Addr()
}
