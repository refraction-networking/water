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

// NewRelay creates a relay with the given Config without starting
// it. To start the relay, call [RelayTo] or [ListenAndRelayTo].
//
// Deprecated: use NewRelayWithContext instead.
func NewRelay(c *water.Config) (water.Relay, error) {
	return NewRelayWithContext(context.Background(), c)
}

// NewRelayWithContext creates a relay with the given Config and
// context without starting it. To start the relay, call [RelayTo]
// or [ListenAndRelayTo].
func NewRelayWithContext(ctx context.Context, c *water.Config) (water.Relay, error) {
	return &Relay{
		config:  c.Clone(),
		ctx:     ctx,
		running: new(atomic.Bool),
	}, nil
}

// RelayTo implements Relay.RelayTo().
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

// ListenAndRelayTo implements Relay.ListenAndRelayTo().
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

func (r *Relay) Close() error {
	if !r.running.CompareAndSwap(true, false) {
		return nil
	}

	if r.config != nil {
		return r.config.NetworkListener.Close()
	}

	return fmt.Errorf("water: relay is not configured")
}

// Addr implements Relay.Addr().
func (r *Relay) Addr() net.Addr {
	if r.config == nil {
		return nil
	}

	return r.config.NetworkListener.Addr()
}
