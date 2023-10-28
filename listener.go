package water

import (
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

type newListenerFunc func(*Config) (Listener, error)

var (
	knownListenerVersions = make(map[string]newListenerFunc)

	ErrListenerAlreadyRegistered = errors.New("water: listener already registered")
	ErrListenerVersionNotFound   = errors.New("water: listener version not found")
	ErrUnimplementedListener     = errors.New("water: unimplemented Listener")

	_ Listener = (*UnimplementedListener)(nil) // type guard
)

// UnimplementedListener is a Listener that always returns errors.
//
// It is used to ensure forward compatibility of the Listener interface.
type UnimplementedListener struct{}

// Accept implements net.Listener.Accept().
func (*UnimplementedListener) Accept() (net.Conn, error) {
	return nil, ErrUnimplementedListener
}

// Close implements net.Listener.Close().
func (*UnimplementedListener) Close() error {
	return ErrUnimplementedListener
}

// Addr implements net.Listener.Addr().
func (*UnimplementedListener) Addr() net.Addr {
	return nil
}

// AcceptWATER implements water.Listener.AcceptWATER().
func (*UnimplementedListener) AcceptWATER() (Conn, error) {
	return nil, ErrUnimplementedListener
}

// mustEmbedUnimplementedListener is a function that developers cannot
func (*UnimplementedListener) mustEmbedUnimplementedListener() {}

// RegisterListener registers a Listener function for the given version to
// the global registry. Only registered versions can be recognized and
// used by NewListener().
func RegisterListener(version string, listener newListenerFunc) error {
	if _, ok := knownListenerVersions[version]; ok {
		return ErrListenerAlreadyRegistered
	}
	knownListenerVersions[version] = listener
	return nil
}

// NewListener creates a new Listener from the config.
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
func NewListener(c *Config) (Listener, error) {
	core, err := NewCore(c)
	if err != nil {
		return nil, err
	}

	// Search through all exported names and match them to potential
	// Listener versions.
	//
	// TODO: detect the version of the WebAssembly Transport Module
	// in a more organized way.
	for _, export := range core.Module().Exports() {
		if f, ok := knownListenerVersions[export.Name()]; ok {
			return f(c)
		}
	}

	return nil, ErrListenerVersionNotFound
}
