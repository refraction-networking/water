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
	running *atomic.Bool

	dialNetwork, dialAddress string

	water.UnimplementedRelay // embedded to ensure forward compatibility
}

// NewRelay creates a relay with the given Config without starting
// it. To start the relay, call Start().
func NewRelay(c *water.Config) (water.Relay, error) {
	return &Relay{
		config:  c.Clone(),
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
		core, err = water.NewCore(r.config)
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
		core, err = water.NewCore(r.config)
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
	if r.running.CompareAndSwap(true, false) {
		return nil
	}

	if r.config != nil {
		r.config.NetworkListener.Close()
	}

	return nil
}

// Addr implements Relay.Addr().
func (r *Relay) Addr() net.Addr {
	if r.config == nil {
		return nil
	}

	return r.config.NetworkListener.Addr()
}
