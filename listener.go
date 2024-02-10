package water

import (
	"context"
	"errors"
	"net"
)

// Listener listens on a local network address and upon caller
// calling Accept(), it accepts an incoming connection and
// returns the net.Conn upgraded by the WebAssembly Transport
// Module.
//
// The structure of a Listener is as follows:
//
//	            +---------------+ accept +---------------+ accept
//	       ---->|               |------->|     Downgrade |------->
//	Source      |  net.Listener |        |  WebAssembly  |         Caller
//	       <----|               |<-------| Upgrade       |<-------
//	            +---------------+        +---------------+
//	                     \                      /
//	                      \------Listener------/
//
// As shown above, a Listener consists of a net.Listener to accept
// incoming connections and a WATM to handle the incoming connections
// from an external source. Accept() returns a net.Conn that caller may
// Read()-from or Write()-to.
type Listener interface {
	// Listener implements net.Listener
	net.Listener

	// AcceptWATER waits for and returns the next connection to the listener
	// as a water.Conn.
	AcceptWATER() (Conn, error)

	mustEmbedUnimplementedListener()
}

type newListenerFunc func(context.Context, *Config) (Listener, error)

var (
	knownListenerVersions = make(map[string]newListenerFunc)

	ErrListenerAlreadyRegistered = errors.New("water: listener already registered")
	ErrListenerVersionNotFound   = errors.New("water: listener version not found")
	ErrUnimplementedListener     = errors.New("water: unimplemented Listener")
)

// UnimplementedListener is a Listener that always returns errors.
//
// It is used to ensure forward compatibility of the Listener interface.
type UnimplementedListener struct{}

// AcceptWATER implements water.Listener.AcceptWATER().
func (*UnimplementedListener) AcceptWATER() (Conn, error) {
	return nil, ErrUnimplementedListener
}

// mustEmbedUnimplementedListener is a function that developers cannot
func (*UnimplementedListener) mustEmbedUnimplementedListener() {} //nolint:unused

// RegisterWATMListener is a function used by Transport Module drivers
// (e.g., `transport/v0`) to register a function that spawns a new [Listener]
// from a given [Config] for a specific version. Renamed from RegisterListener.
//
// This is not a part of WATER API and should not be used by developers
// wishing to integrate WATER into their applications.
func RegisterWATMListener(version string, listener newListenerFunc) error {
	if _, ok := knownListenerVersions[version]; ok {
		return ErrListenerAlreadyRegistered
	}
	knownListenerVersions[version] = listener
	return nil
}

// NewListener creates a new [Listener] from the given [Config].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// Deprecated: use [NewListenerWithContext] instead.
func NewListener(c *Config) (Listener, error) {
	return NewListenerWithContext(context.Background(), c)
}

// NewListenerWithContext creates a new [Listener] from the [Config] with
// the given [context.Context].
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
//
// The context is passed to [NewCoreWithContext] and the registered versioned
// listener creation function to control the lifetime of the call to function
// calls into the WebAssembly module.
// If the context is canceled or reaches its deadline, any current and future
// function call will return with an error.
// Call [WazeroRuntimeConfigFactory.SetCloseOnContextDone] with false to disable
// this behavior.
func NewListenerWithContext(ctx context.Context, c *Config) (Listener, error) {
	core, err := NewCoreWithContext(context.Background(), c)
	if err != nil {
		return nil, err
	}

	// Search through all exported names and match them to potential
	// Listener versions.
	//
	// TODO: detect the version of the WebAssembly Transport Module
	// in a more organized way.
	for exportName := range core.Exports() {
		if f, ok := knownListenerVersions[exportName]; ok {
			return f(ctx, c)
		}
	}

	return nil, ErrListenerVersionNotFound
}
