//go:build !exclude_v0

package v0

import (
	"context"
	"fmt"

	"github.com/gaukas/water"
)

func init() {
	water.RegisterDialer("_water_v0", NewDialer)
}

// Dialer implements water.Dialer utilizing Water WATM API v0.
type Dialer struct {
	config *water.Config

	water.UnimplementedDialer // embedded to ensure forward compatibility
}

// NewDialer creates a new Dialer.
func NewDialer(c *water.Config) (water.Dialer, error) {
	return &Dialer{
		config: c.Clone(),
	}, nil
}

// Dial dials the network address using the dialerFunc specified in config.
//
// Dial interfaces water.Dialer.
func (d *Dialer) Dial(network, address string) (conn water.Conn, err error) {
	return d.DialContext(context.Background(), network, address)
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn water.Conn, err error) {
	if d.config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}

	ctxReady, dialReady := context.WithCancel(context.Background())
	go func() {
		defer dialReady()
		var core water.Core
		core, err = water.NewCore(d.config)
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
