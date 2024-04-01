package water

import (
	"context"
	"errors"
)

// Connector acts like a dialer, despite the fact that the destination is managed by
// the WebAssembly Transport Module (WATM) instead of specified by the caller.
//
// In other words, Connector is a dialer that does not take network or address as input
// but returns a connection to a remote network address specified by the WATM.
type Connector interface {
	// Connect dials a remote network address provided by the WATM
	// and returns a superset of net.Conn.
	//
	// It is recommended to use ConnectContext instead of Connect. This
	// method may be removed in the future.
	Connect() (Conn, error)

	// ConnectContext dials a remote network address provided by the WATM
	// with the given context and returns a superset of net.Conn.
	ConnectContext(ctx context.Context) (Conn, error)

	mustEmbedUnimplementedConnector()
}

type newConnectorFunc func(context.Context, *Config) (Connector, error)

var (
	knownConnectorVersions = make(map[string]newConnectorFunc)

	ErrConnectorAlreadyRegistered = errors.New("water: free dialer already registered")
	ErrConnectorVersionNotFound   = errors.New("water: free dialer version not found")
	ErrUnimplementedConnector     = errors.New("water: unimplemented free dialer")

	_ Connector = (*UnimplementedConnector)(nil) // type guard
)

// UnimplementedConnector is a Connector that always returns errors.
//
// It is used to ensure forward compatibility of the Connector interface.
type UnimplementedConnector struct{}

// Connect implements Connector.Connect().
func (*UnimplementedConnector) Connect() (Conn, error) {
	return nil, ErrUnimplementedConnector
}

// ConnectContext implements Connector.ConnectContext().
func (*UnimplementedConnector) ConnectContext(_ context.Context) (Conn, error) {
	return nil, ErrUnimplementedConnector
}

func (*UnimplementedConnector) mustEmbedUnimplementedConnector() {} //nolint:unused

func RegisterWATMConnector(name string, dialer newConnectorFunc) error {
	if _, ok := knownConnectorVersions[name]; ok {
		return ErrConnectorAlreadyRegistered
	}
	knownConnectorVersions[name] = dialer
	return nil
}

func NewConnectorWithContext(ctx context.Context, cfg *Config) (Connector, error) {
	core, err := NewCoreWithContext(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Sniff the version of the dialer
	for exportName := range core.Exports() {
		if f, ok := knownConnectorVersions[exportName]; ok {
			return f(ctx, cfg)
		}
	}

	return nil, ErrConnectorVersionNotFound
}
