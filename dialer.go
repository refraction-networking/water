package water

import (
	"context"
	"errors"
)

// Dialer dials a remote network address upon caller calling
// Dial() and returns a net.Conn which is upgraded by the
// WebAssembly Transport Module.
//
// The structure of a Dialer is as follows:
//
//	        dial +----------------+ dial
//	       ----->|        Upgrade |------>
//	Caller       |   WebAssembly  |        Remote
//	       <-----| Downgrade      |<------
//	             +----------------+
//	                   Dialer
type Dialer interface {
	// Dial dials the remote network address and returns a
	// superset of net.Conn.
	//
	// It is recommended to use DialContext instead of Dial. This
	// method may be removed in the future.
	Dial(network, address string) (Conn, error)

	// DialContext dials the remote network address with the given context
	// and returns a superset of net.Conn.
	DialContext(ctx context.Context, network, address string) (Conn, error)

	mustEmbedUnimplementedDialer()
}

type newDialerFunc func(context.Context, *Config) (Dialer, error)

var (
	knownDialerVersions = make(map[string]newDialerFunc)

	ErrDialerAlreadyRegistered = errors.New("water: dialer already registered")
	ErrDialerVersionNotFound   = errors.New("water: dialer version not found")
	ErrUnimplementedDialer     = errors.New("water: unimplemented dialer")

	_ Dialer = (*UnimplementedDialer)(nil) // type guard
)

// UnimplementedDialer is a Dialer that always returns errors.
//
// It is used to ensure forward compatibility of the Dialer interface.
type UnimplementedDialer struct{}

// Dial implements Dialer.Dial().
func (*UnimplementedDialer) Dial(_, _ string) (Conn, error) {
	return nil, ErrUnimplementedDialer
}

// DialContext implements Dialer.DialContext().
func (*UnimplementedDialer) DialContext(_ context.Context, _, _ string) (Conn, error) {
	return nil, ErrUnimplementedDialer
}

// mustEmbedUnimplementedDialer is a function that developers cannot
// manually implement. It is used to ensure forward compatibility of
// the Dialer interface.
func (*UnimplementedDialer) mustEmbedUnimplementedDialer() {} //nolint:unused

// RegisterWATMDialer is a function used by Transport Module drivers
// (e.g., `transport/v0`) to register a function that spawns a new [Dialer]
// from a given [Config] for a specific version. Renamed from RegisterDialer.
//
// This is not a part of WATER API and should not be used by developers
// wishing to integrate WATER into their applications.
func RegisterWATMDialer(version string, dialer newDialerFunc) error {
	if _, ok := knownDialerVersions[version]; ok {
		return ErrDialerAlreadyRegistered
	}
	knownDialerVersions[version] = dialer
	return nil
}

// NewDialer creates a new [Dialer] from the given [Config].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// Deprecated: use NewDialerWithContext instead.
func NewDialer(c *Config) (Dialer, error) {
	return NewDialerWithContext(context.Background(), c)
}

// NewDialerWithContext creates a new [Dialer] from the [Config] with
// the given [context.Context].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// The context is passed to [NewCoreWithContext] and the registered versioned
// dialer creation function to control the lifetime of the call to function
// calls into the WebAssembly module.
// If the context is canceled or reaches its deadline, any current and future
// function call will return with an error.
// Call [WazeroRuntimeConfigFactory.SetCloseOnContextDone] with false to disable
// this behavior.
//
// The context SHOULD be used as the default context for call to [Dialer.Dial]
// by the dialer implementation.
func NewDialerWithContext(ctx context.Context, c *Config) (Dialer, error) {
	core, err := NewCoreWithContext(ctx, c)
	if err != nil {
		return nil, err
	}

	// Search through all exported names and match them to potential
	// Dialer versions.
	//
	// TODO: detect the version of the WebAssembly Transport Module
	// in a more organized way.
	for exportName := range core.Exports() {
		if f, ok := knownDialerVersions[exportName]; ok {
			return f(ctx, c)
		}
	}

	return nil, ErrDialerVersionNotFound
}

// FixedDialer acts like a dialer, despite the fact that the destination is managed by
// the WebAssembly Transport Module (WATM) instead of specified by the caller.
//
// In other words, FixedDialer is a dialer that does not take network or address as input
// but returns a connection to a remote network address specified by the WATM.
type FixedDialer interface {
	// DialFixed dials a remote network address provided by the WATM
	// and returns a superset of net.Conn.
	//
	// It is recommended to use DialFixedContext instead of Connect. This
	// method may be removed in the future.
	DialFixed() (Conn, error)

	// DialFixedContext dials a remote network address provided by the WATM
	// with the given context and returns a superset of net.Conn.
	DialFixedContext(ctx context.Context) (Conn, error)

	mustEmbedUnimplementedFixedDialer()
}

type newFixedDialerFunc func(context.Context, *Config) (FixedDialer, error)

var (
	knownFixedDialerVersions = make(map[string]newFixedDialerFunc)

	ErrFixedDialerAlreadyRegistered = errors.New("water: free dialer already registered")
	ErrFixedDialerVersionNotFound   = errors.New("water: free dialer version not found")
	ErrUnimplementedFixedDialer     = errors.New("water: unimplemented free dialer")

	_ FixedDialer = (*UnimplementedFixedDialer)(nil) // type guard
)

// UnimplementedFixedDialer is a FixedDialer that always returns errors.
//
// It is used to ensure forward compatibility of the FixedDialer interface.
type UnimplementedFixedDialer struct{}

// Connect implements FixedDialer.DialFixed().
func (*UnimplementedFixedDialer) DialFixed() (Conn, error) {
	return nil, ErrUnimplementedFixedDialer
}

// DialFixedContext implements FixedDialer.DialFixedContext().
func (*UnimplementedFixedDialer) DialFixedContext(_ context.Context) (Conn, error) {
	return nil, ErrUnimplementedFixedDialer
}

func (*UnimplementedFixedDialer) mustEmbedUnimplementedFixedDialer() {} //nolint:unused

func RegisterWATMFixedDialer(name string, dialer newFixedDialerFunc) error {
	if _, ok := knownFixedDialerVersions[name]; ok {
		return ErrFixedDialerAlreadyRegistered
	}
	knownFixedDialerVersions[name] = dialer
	return nil
}

func NewFixedDialerWithContext(ctx context.Context, cfg *Config) (FixedDialer, error) {
	core, err := NewCoreWithContext(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Sniff the version of the dialer
	for exportName := range core.Exports() {
		if f, ok := knownFixedDialerVersions[exportName]; ok {
			return f(ctx, cfg)
		}
	}

	return nil, ErrFixedDialerVersionNotFound
}
