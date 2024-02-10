package v0

import (
	"net"
)

// ManagedDialer restricts the network and address to be
// used by the dialerFunc.
type ManagedDialer struct {
	network    string
	address    string
	dialerFunc func(network, address string) (net.Conn, error)
	// mapFdConn       map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors! (So we could close them when needed)
	// mapFdClonedFile map[int32]*os.File // saves all files so GC won't close them
}

// NewManagedDialer creates a new ManagedDialer.
func NewManagedDialer(network, address string, dialerFunc func(network, address string) (net.Conn, error)) *ManagedDialer {
	return &ManagedDialer{
		network:    network,
		address:    address,
		dialerFunc: dialerFunc,
	}
}

// Dial dials the network address using the dialerFunc of the ManagedDialer.
func (md *ManagedDialer) Dial() (net.Conn, error) {
	return md.dialerFunc(md.network, md.address)
}
