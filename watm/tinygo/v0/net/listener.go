package net

import "github.com/refraction-networking/water/watm/wasip1"

// Listener is the interface for a generic network listener.
type Listener interface {
	Accept() (Conn, error)
}

// type guard: *TCPListener must implement Listener
var _ Listener = (*TCPListener)(nil)

// TCPListener is a fake TCP listener which calls to the host
// to accept a connection.
//
// By saying "fake", it means that the file descriptor is not
// managed inside the WATM, but by the host application.
type TCPListener struct {
}

func (l *TCPListener) Accept() (Conn, error) {
	return HostManagedAccept()
}

// HostManagedAccept asks the host to accept a connection.
func HostManagedAccept() (Conn, error) {
	fd, err := wasip1.DecodeWATERError(_import_host_accept())
	if err != nil {
		return nil, err
	}

	return RebuildTCPConn(fd), nil
}
