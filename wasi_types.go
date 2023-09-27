package water

import "fmt"

// WASINetwork specifies the network types requested by the WASM module.
// The WASM module can use this to specify a preferred network type.
type WASINetwork int32

// Preset WASINetwork
const (
	WASI_NETWORK_TCP WASINetwork = iota // default/unspecified/0 -> TCP
	WASI_NETWORK_UDP                    // 1 -> UDP
)

var mapWASINetworkNames = map[WASINetwork]string{
	WASI_NETWORK_TCP: "tcp",
	WASI_NETWORK_UDP: "udp",
}

func (n WASINetwork) String() string {
	if name, ok := mapWASINetworkNames[n]; ok {
		return name
	}
	panic(fmt.Sprintf("water: unknown WASINetwork: %d", n))
}
