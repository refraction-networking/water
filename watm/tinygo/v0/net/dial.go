package net

import (
	"github.com/refraction-networking/water/watm/wasip1"
)

// Dial dials a remote host for a network connection.
//
// In v0, network and address parameters are ignored, and
// it is internally called as [HostManagedDial].
func Dial(_, _ string) (Conn, error) {
	return HostManagedDial()
}

// HostManagedDial asks the host to dial a remote host predefined
// by the host.
func HostManagedDial() (Conn, error) {
	fd, err := wasip1.DecodeWATERError(_import_host_dial())
	if err != nil {
		return nil, err
	}

	return RebuildTCPConn(fd), nil
}
