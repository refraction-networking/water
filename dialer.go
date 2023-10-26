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
	Dial(network, address string) (Conn, error)
	DialContext(ctx context.Context, network, address string) (Conn, error)

	mustEmbedUnimplementedDialer()
}

type newDialerFunc func(*Config) (Dialer, error)

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
func (*UnimplementedDialer) mustEmbedUnimplementedDialer() {}

// RegisterDialer registers a dialer function for the given version to
// the global registry. Only registered versions can be recognized and
// used by NewDialer().
func RegisterDialer(version string, dialer newDialerFunc) error {
	if _, ok := knownDialerVersions[version]; ok {
		return ErrDialerAlreadyRegistered
	}
	knownDialerVersions[version] = dialer
	return nil
}

// NewDialer creates a new Dialer from the config.
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
func NewDialer(c *Config) (Dialer, error) {
	core, err := NewCore(c)
	if err != nil {
		return nil, err
	}

	// Search through all exported names and match them to potential
	// Dialer versions.
	//
	// TODO: detect the version of the WebAssembly Transport Module
	// in a more organized way.
	for _, export := range core.Module().Exports() {
		if f, ok := knownDialerVersions[export.Name()]; ok {
			return f(c)
		}
	}

	return nil, ErrDialerVersionNotFound
}
