package water

import (
	"fmt"
	"net"
)

// TODO: allow WASM instance to request the network connection
// to be wrapped with an application protocol. That is, any bytes
// received from the network connection will be decoded by the
// application protocol and any bytes written to the network
// connection will be encoded by the application protocol.
type WASIApplicationProtocol = int32

const (
	WASI_AP_NONE WASIApplicationProtocol = iota
	WASI_AP_TLS_CLIENT
	WASI_AP_TLS_SERVER
)

// WASIApplicationProtocolWrapper is an interface for a wrapper
// that wraps a net.Conn with an application protocol.
type WASIApplicationProtocolWrapper interface {
	Wrap(WASIApplicationProtocol, net.Conn) (net.Conn, error)
}

// noWASIApplicationProtocolWrapper is a WASIApplicationProtocolWrapper
// that does not wrap the net.Conn with any application protocol.
// In other words, it returns the net.Conn as is.
type noWASIApplicationProtocolWrapper struct{}

func (noWASIApplicationProtocolWrapper) Wrap(ap WASIApplicationProtocol, conn net.Conn) (net.Conn, error) {
	if ap != WASI_AP_NONE {
		return nil, fmt.Errorf("water: no application protocol wrapper is available")
	}
	return conn, nil
}

// TODO: implement defaultWASIApplicationProtocolWrapper to support a few
// popular application protocols, OR completely get rid of this feature.
