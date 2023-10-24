//go:build !exclude_v0

package v0

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/gaukas/water"
)

func init() {
	err := water.RegisterRelay("_water_v0", NewRelay)
	if err != nil {
		panic(err)
	}
}

// Relay implements water.Relay utilizing Water WATM API v0.
type Relay struct {
	config  *water.Config
	started *atomic.Bool
	closed  *atomic.Bool

	dialNetwork, dialAddress string

	water.UnimplementedRelay // embedded to ensure forward compatibility
}

// NewRelay creates a relay with the given Config without starting
// it. To start the relay, call Start().
func NewRelay(c *water.Config) (water.Relay, error) {
	return &Relay{
		config:  c.Clone(),
		started: new(atomic.Bool),
		closed:  new(atomic.Bool),
	}, nil
}

// RelayTo implements Relay.RelayTo().
func (r *Relay) RelayTo(network, address string) error {
	if r.started.Load() {
		return water.ErrRelayAlreadyStarted
	}

	if r.config == nil {
		return fmt.Errorf("water: relaying with nil config is not allowed")
	}

	r.dialNetwork = network
	r.dialAddress = address

	var core water.Core
	var err error
	for !r.closed.Load() {
		core, err = water.NewCore(r.config)
		if err != nil {
			return err
		}

		_, err = relay(core, network, address)
		if err != nil {
			if !r.closed.Load() { // errored before closing
				return err
			}
			break
		}
	}

	return nil
}

// ListenAndRelayTo implements Relay.ListenAndRelayTo().
func (r *Relay) ListenAndRelayTo(lnetwork, laddress, rnetwork, raddress string) error {
	if r.started.Load() {
		return water.ErrRelayAlreadyStarted
	}

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
	for !r.closed.Load() {
		core, err = water.NewCore(r.config)
		if err != nil {
			return err
		}

		_, err = relay(core, rnetwork, raddress)
		if err != nil {
			if !r.closed.Load() { // errored before closing
				return err
			}
			break
		}
	}

	return nil
}

func (r *Relay) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}

	if r.config != nil {
		r.config.NetworkListener.Close()
	}

	return nil
}
