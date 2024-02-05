package water

import (
	"net"
)

// Conn is an abstracted connection interface which is expected
// to encapsulate a Core.
type Conn interface {
	net.Conn

	// For forward compatibility with any new methods added to the
	// interface, all Conn implementations MUST embed the
	// UnimplementedConn in order to make sure they could be used
	// in the future without any code change.
	mustEmbedUnimplementedConn()
}

// UnimplementedConn is used to provide forward compatibility for
// implementations of Conn, such that if new methods are added
// to the interface, old implementations will not be required to implement
// each of them.
type UnimplementedConn struct{}

// mustEmbedUnimplementedConn is a no-op method used to test an implementation
// of Conn really embeds UnimplementedConn.
func (*UnimplementedConn) mustEmbedUnimplementedConn() {} //nolint:unused
