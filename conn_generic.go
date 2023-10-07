package water

import (
	"fmt"
	"net"
	"time"
)

var (
	mapCoreDialContext = make(map[string]func(core *core, network, address string) (Conn, error))
	mapCoreAccept      = make(map[string]func(*core) (Conn, error))
)

// Conn is an abstracted connection interface which encapsulates
// a WASM runtime core.
type Conn interface {
	net.Conn

	// For forward compatibility with any new methods added to the
	// interface, all Conn implementations MUST embed the
	// UnimplementedConn in order to make sure they could be used
	// in the future without any code change.
	mustEmbedUnimplementedConn()
}

func RegisterDial(version string, dialContext func(core *core, network, address string) (Conn, error)) error {
	if _, ok := mapCoreDialContext[version]; ok {
		return fmt.Errorf("water: core dial context already registered for version %s", version)
	}
	mapCoreDialContext[version] = dialContext
	return nil
}

func RegisterAccept(version string, accept func(*core) (Conn, error)) error {
	if _, ok := mapCoreAccept[version]; ok {
		return fmt.Errorf("water: core accept already registered for version %s", version)
	}
	mapCoreAccept[version] = accept
	return nil
}

// UnimplementedConn is used to provide forward compatibility for
// implementations of Conn, such that if new methods are added
// to the interface, old implementations will not be required to implement
// each of them.
type UnimplementedConn struct{}

func (*UnimplementedConn) Read([]byte) (int, error) {
	return 0, fmt.Errorf("water: Read() is not implemented")
}

func (*UnimplementedConn) Write([]byte) (int, error) {
	return 0, fmt.Errorf("water: Write() is not implemented")
}

func (*UnimplementedConn) Close() error {
	return fmt.Errorf("water: Close() is not implemented")
}

func (*UnimplementedConn) LocalAddr() net.Addr {
	return nil
}

func (*UnimplementedConn) RemoteAddr() net.Addr {
	return nil
}

func (*UnimplementedConn) SetDeadline(_ time.Time) error {
	return fmt.Errorf("water: SetDeadline() is not implemented")
}

func (*UnimplementedConn) SetReadDeadline(_ time.Time) error {
	return fmt.Errorf("water: SetReadDeadline() is not implemented")
}

func (*UnimplementedConn) SetWriteDeadline(_ time.Time) error {
	return fmt.Errorf("water: SetWriteDeadline() is not implemented")
}

func (*UnimplementedConn) mustEmbedUnimplementedConn() {}

var _ Conn = (*UnimplementedConn)(nil)
