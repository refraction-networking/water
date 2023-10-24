package water

import "errors"

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

	mustEmbedUnimplementedRelay()
}

type newRelayFunc func(*Config) (Relay, error)

var (
	knownRelayVersions = make(map[string]newRelayFunc)

	ErrRelayAlreadyRegistered = errors.New("water: relay already registered")
	ErrRelayVersionNotFound   = errors.New("water: relay version not found")
	ErrUnimplementedRelay     = errors.New("water: unimplemented relay")

	ErrRelayAlreadyStarted = errors.New("water: relay already started") // RelayTo and ListenAndRelayTo may return this error if a relay was reused.
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

// mustEmbedUnimplementedRelay is a function that developers cannot
// manually implement. It is used to ensure forward compatibility of
// the Relay interface.
func (*UnimplementedRelay) mustEmbedUnimplementedRelay() {}

// RegisterRelay registers a relay function for the given version to
// the global registry. Only registered versions can be recognized and
// used by NewRelay().
func RegisterRelay(version string, relay newRelayFunc) error {
	if _, ok := knownRelayVersions[version]; ok {
		return ErrRelayAlreadyRegistered
	}
	knownRelayVersions[version] = relay
	return nil
}

// NewRelay creates a new Relay from the config.
//
// It automatically detects the version of the WebAssembly Transport
// Module specified in the config.
func NewRelay(c *Config) (Relay, error) {
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
		if f, ok := knownRelayVersions[export.Name()]; ok {
			return f(c)
		}
	}

	return nil, ErrRelayVersionNotFound
}
