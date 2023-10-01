package water

import (
	"fmt"
	"net"
)

var mapOBRCV = make(map[int32]func(*runtimeCore) (RuntimeConn, error)) // OutboundRuntimeConnWithVersion
var mapIBRCV = make(map[int32]func(*runtimeCore) (RuntimeConn, error)) // InboundRuntimeConnWithVersion
var mapRLRCV = make(map[int32]func(*runtimeCore) (RuntimeConn, error)) // RelayingRuntimeConnWithVersion

// RuntimeConn is an interface for a Conn-like object that encapsulates
// a WASM runtime core.
// All versions of RuntimeConn must implement this interface.
type RuntimeConn interface {
	net.Conn
}

// OutboundRuntimeConnWithVersion spins up a RuntimeConn of the corresponding version with the
// given core and (implicitly) initializes it.
func OutboundRuntimeConnWithVersion(core *runtimeCore, version int32) (RuntimeConn, error) {
	if f, ok := mapOBRCV[version]; !ok {
		return nil, fmt.Errorf("water: unknown version: %d", version)
	} else {
		return f(core)
	}
}

// RegisterOutboundRuntimeConnWithVersion registers a function that spins up a RuntimeConn of the
// corresponding version with the given core and (implicitly) initializes it.
func RegisterOutboundRuntimeConnWithVersion(version int32, f func(*runtimeCore) (RuntimeConn, error)) {
	if _, ok := mapOBRCV[version]; ok {
		panic(fmt.Sprintf("water: version %d already registered", version))
	}
	mapOBRCV[version] = f
}

// InboundRuntimeConnWithVersion spins up a RuntimeConn of the corresponding version with the
// given core and (implicitly) initializes it.
func InboundRuntimeConnWithVersion(core *runtimeCore, version int32) (RuntimeConn, error) {
	if f, ok := mapIBRCV[version]; !ok {
		return nil, fmt.Errorf("water: unknown version: %d", version)
	} else {
		return f(core)
	}
}

// RegisterInboundRuntimeConnWithVersion registers a function that spins up a RuntimeConn of the
// corresponding version with the given core and (implicitly) initializes it.
func RegisterInboundRuntimeConnWithVersion(version int32, f func(*runtimeCore) (RuntimeConn, error)) {
	if _, ok := mapIBRCV[version]; ok {
		panic(fmt.Sprintf("water: version %d already registered", version))
	}
	mapIBRCV[version] = f
}

// RelayingRuntimeConnWithVersion spins up a RuntimeConn of the corresponding version with the
// given core and (implicitly) initializes it.
func RelayingRuntimeConnWithVersion(core *runtimeCore, version int32) (RuntimeConn, error) {
	if f, ok := mapRLRCV[version]; !ok {
		return nil, fmt.Errorf("water: unknown version: %d", version)
	} else {
		return f(core)
	}
}

// RegisterRelayingRuntimeConnWithVersion registers a function that spins up a RuntimeConn of the
// corresponding version with the given core and (implicitly) initializes it.
func RegisterRelayingRuntimeConnWithVersion(version int32, f func(*runtimeCore) (RuntimeConn, error)) {
	if _, ok := mapRLRCV[version]; ok {
		panic(fmt.Sprintf("water: version %d already registered", version))
	}
	mapRLRCV[version] = f
}
