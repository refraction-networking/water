package water

import (
	"context"
	"errors"
	"net"
)

// Relay listens on a local network address and handles requests
// on incoming connections by passing the incoming connection to
// the WebAssembly Transport Module and dial corresponding
// outbound connections to a pre-defined destination address.
// By doing so, WATM upgrades the incoming connection.
//
// The structure of a Relay is as follows:
//
//	        accept +---------------+      +---------------+ dial
//	       ------->|               |----->|       Upgrade |----->
//	Source         |  net.Listener |      |  WebAssembly  |       Remote
//	       <-------|               |<-----| Downgrade     |<-----
//	               +---------------+      +---------------+
//	                        \                    /
//	                         \------Relay-------/
type Relay interface {
	// RelayTo relays the incoming connection to the address specified
	// by network and address.
	RelayTo(network, address string) error

	// ListenAndRelayTo listens on the local network address and relays
	// the incoming connection to the address specified by rnetwork
	// and raddress.
	ListenAndRelayTo(lnetwork, laddress, rnetwork, raddress string) error

	// Close closes the relay. No further incoming connections will be
	// accepted and no further outbound connections will be dialed. It
	// does not close the established connections.
	Close() error

	// Addr returns the local address the relay is listening on.
	//
	// If no address is available, instead of panicking it returns nil.
	Addr() net.Addr

	// Shutdown terminates all established connections and stops the relay.
	Shutdown() error

	mustEmbedUnimplementedRelay()
}

type newRelayFunc func(context.Context, *Config) (Relay, error)

var (
	knownRelayVersions = make(map[string]newRelayFunc)

	ErrRelayAlreadyRegistered = errors.New("water: relay already registered")
	ErrRelayVersionNotFound   = errors.New("water: relay version not found")
	ErrUnimplementedRelay     = errors.New("water: unimplemented relay")
	ErrRelayAlreadyStarted    = errors.New("water: relay already started") // RelayTo and ListenAndRelayTo may return this error if a relay was reused.

	_ Relay = (*UnimplementedRelay)(nil) // type guard
)

// UnimplementedRelay is a Relay that always returns errors.
//
// It is used to ensure forward compatibility of the Relay interface.
type UnimplementedRelay struct{}

// RelayTo implements Relay.RelayTo().
func (*UnimplementedRelay) RelayTo(_, _ string) error {
	return ErrUnimplementedRelay
}

// ListenAndRelayTo implements Relay.ListenAndRelayTo().
func (*UnimplementedRelay) ListenAndRelayTo(_, _, _, _ string) error {
	return ErrUnimplementedRelay
}

// Close implements Relay.Close().
func (*UnimplementedRelay) Close() error {
	return ErrUnimplementedRelay
}

// Addr implements Relay.Addr().
func (*UnimplementedRelay) Addr() net.Addr {
	return nil
}

// Shutdown implements Relay.Shutdown().
func (*UnimplementedRelay) Shutdown() error {
	return ErrUnimplementedRelay
}

// mustEmbedUnimplementedRelay is a function that developers cannot
// manually implement. It is used to ensure forward compatibility of
// the Relay interface.
func (*UnimplementedRelay) mustEmbedUnimplementedRelay() {} //nolint:unused

// RegisterWATMRelay is a function used by Transport Module drivers
// (e.g., `transport/v0`) to register a function that spawns a new [Relay]
// from a given [Config] for a specific version. Renamed from RegisterRelay.
//
// This is not a part of WATER API and should not be used by developers
// wishing to integrate WATER into their applications.
func RegisterWATMRelay(version string, relay newRelayFunc) error {
	if _, ok := knownRelayVersions[version]; ok {
		return ErrRelayAlreadyRegistered
	}
	knownRelayVersions[version] = relay
	return nil
}

// NewRelay creates a new [Relay] from the given [Config].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// Deprecated: use [NewRelayWithContext] instead.
func NewRelay(c *Config) (Relay, error) {
	return NewRelayWithContext(context.Background(), c)
}

// NewRelayWithContext creates a new [Relay] from the [Config] with
// the given [context.Context].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// The context is passed to [NewCoreWithContext] and the registered versioned
// relay creation function to control the lifetime of the call to function
// calls into the WebAssembly module.
// If the context is canceled or reaches its deadline, any current and future
// function call will return with an error.
// Call [WazeroRuntimeConfigFactory.SetCloseOnContextDone] with false to disable
// this behavior.
func NewRelayWithContext(ctx context.Context, c *Config) (Relay, error) {
	core, err := NewCoreWithContext(ctx, c)
	if err != nil {
		return nil, err
	}

	// Search through all exported names and match them to potential
	// Listener versions.
	//
	// TODO: detect the version of the WebAssembly Transport Module
	// in a more organized way.
	for exportName := range core.Exports() {
		if f, ok := knownRelayVersions[exportName]; ok {
			return f(ctx, c)
		}
	}

	return nil, ErrRelayVersionNotFound
}
