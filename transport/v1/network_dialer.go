package v1

import (
	"fmt"
	"net"
)

// networkDialer is a dialer used to dial remote network addresses.
// It is used by Dialer and Relay modes.
type networkDialer struct {
	dialerFunc func(network, address string) (net.Conn, error)

	overrideAddress struct {
		network string
		address string
	} // used by DialAny. If not set, DialAny will fail. This address is not checked by addressValidator.

	addressValidator func(network, address string) error // used by Dial, if set. Otherwise all addresses are considered invalid.
}

// Dial dials the network address using the dialerFunc of the networkDialer.
// It validates the address using the addressValidator if set.
//
// It should be used when the caller is aware of the address to dial.
func (nd *networkDialer) Dial(network, address string) (net.Conn, error) {
	// // TODO: maybe use override address if it is set?
	// if nd.HasOverrideAddress() {
	// 	return nd.dialerFunc(nd.overrideAddress.network, nd.overrideAddress.address)
	// }

	if nd.addressValidator == nil { // foolproof: not set == not allowed
		return nil, fmt.Errorf("address validator is not set")
	}

	if err := nd.addressValidator(network, address); err != nil {
		return nil, fmt.Errorf("address validation: %w", err)
	}

	return nd.dialerFunc(network, address)
}

// DialFixed dials the predetermined address using the dialerFunc of the networkDialer.
//
// It should be used only when the caller is not aware of the address to dial.
func (nd *networkDialer) DialFixed() (net.Conn, error) {
	return nd.dialerFunc(nd.overrideAddress.network, nd.overrideAddress.address)
}

func (nd *networkDialer) HasOverrideAddress() bool {
	return nd.overrideAddress.network != "" && nd.overrideAddress.address != ""
}
