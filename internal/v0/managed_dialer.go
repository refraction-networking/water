package v0

import (
	"net"
)

// managedDialer restricts the network and address to be
// used by the dialerFunc.
type managedDialer struct {
	network    string
	address    string
	dialerFunc func(network, address string) (net.Conn, error)
	// mapFdConn       map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors! (So we could close them when needed)
	// mapFdClonedFile map[int32]*os.File // saves all files so GC won't close them
}

func ManagedDialer(network, address string, dialerFunc func(network, address string) (net.Conn, error)) *managedDialer {
	return &managedDialer{
		network:    network,
		address:    address,
		dialerFunc: dialerFunc,
	}
}

// dial(apw i32) -> fd i32
func (md *managedDialer) Dial() (net.Conn, error) {
	return md.dialerFunc(md.network, md.address)
}
