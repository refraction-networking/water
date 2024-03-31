package water

import (
	"context"
	"errors"
)

// DialerManaged acts like a dialer, despite the fact that the destination is managed by
// the WebAssembly Transport Module (WATM) instead of specified by the caller.
//
// In other words, DialerManaged is a dialer that does not take network or address as input
// but returns a connection to a remote network address specified by the WATM.
type DialerManaged interface {
	// DialManaged dials a remote network address provided by the WATM
	// and returns a superset of net.Conn.
	//
	// It is recommended to use DialManagedContext instead of DialManaged. This
	// method may be removed in the future.
	DialManaged() (Conn, error)

	// DialContextManaged dials a remote network address provided by the WATM
	// with the given context and returns a superset of net.Conn.
	DialContextManaged(ctx context.Context) (Conn, error)

	mustEmbedUnimplementedDialerManaged()
}

type newDialerManagedFunc func(context.Context, *Config) (DialerManaged, error)

var (
	knownDialerManagedVersions = make(map[string]newDialerManagedFunc)

	ErrDialerManagedAlreadyRegistered = errors.New("water: free dialer already registered")
	ErrDialerManagedVersionNotFound   = errors.New("water: free dialer version not found")
	ErrUnimplementedDialerManaged     = errors.New("water: unimplemented free dialer")

	_ DialerManaged = (*UnimplementedDialerManaged)(nil) // type guard
)

// UnimplementedDialerManaged is a DialerManaged that always returns errors.
//
// It is used to ensure forward compatibility of the DialerManaged interface.
type UnimplementedDialerManaged struct{}

// DialManaged implements DialerManaged.DialManaged().
func (*UnimplementedDialerManaged) DialManaged() (Conn, error) {
	return nil, ErrUnimplementedDialerManaged
}

// DialManagedContext implements DialerManaged.DialContextManaged().
func (*UnimplementedDialerManaged) DialContextManaged(_ context.Context) (Conn, error) {
	return nil, ErrUnimplementedDialerManaged
}

func (*UnimplementedDialerManaged) mustEmbedUnimplementedDialerManaged() {} //nolint:unused

func RegisterWATMDialerManaged(name string, dialer newDialerManagedFunc) error {
	if _, ok := knownDialerManagedVersions[name]; ok {
		return ErrDialerManagedAlreadyRegistered
	}
	knownDialerManagedVersions[name] = dialer
	return nil
}

func NewWATMDialerManagedWithContext(ctx context.Context, name string, cfg *Config) (DialerManaged, error) {
	core, err := NewCoreWithContext(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Sniff the version of the dialer
	for exportName := range core.Exports() {
		if f, ok := knownDialerManagedVersions[exportName]; ok {
			return f(ctx, cfg)
		}
	}

	return nil, ErrDialerManagedVersionNotFound
}
