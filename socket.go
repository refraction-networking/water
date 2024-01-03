package water

import (
	"fmt"
	"net"
	"os"
)

// InsertConn implements Core.
func (c *core) InsertConn(conn net.Conn) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert TCPConn before instantiation")
	}

	switch conn := conn.(type) {
	case *net.TCPConn:
		key, ok := c.instance.InsertTCPConn(conn)
		if !ok {
			return 0, fmt.Errorf("water: (*wazero.Module).InsertTCPConn returned false")
		}
		if key <= 0 {
			return key, fmt.Errorf("water: (*wazero.Module).InsertTCPConn returned invalid key")
		}
		return key, nil
	default:
		// TODO: support other types of connections as much as possible
		return 0, fmt.Errorf("water: unsupported connection type: %T", conn)
	}
}

// InsertListener implements Core.
func (c *core) InsertListener(listener net.Listener) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert TCPListener before instantiation")
	}

	switch listener := listener.(type) {
	case *net.TCPListener:
		key, ok := c.instance.InsertTCPListener(listener)
		if !ok {
			return 0, fmt.Errorf("water: (*wazero.Module).InsertTCPListener returned false")
		}
		if key <= 0 {
			return key, fmt.Errorf("water: (*wazero.Module).InsertTCPListener returned invalid key")
		}
		return key, nil
	default:
		// TODO: support other types of listeners as much as possible
		return 0, fmt.Errorf("water: unsupported listener type: %T", listener)
	}
}

// InsertFile implements Core.
func (c *core) InsertFile(osFile *os.File) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert File before instantiation")
	}

	key, ok := c.instance.InsertOSFile(osFile)
	if !ok {
		return 0, fmt.Errorf("water: (*wazero.Module).InsertFile returned false")
	}
	if key <= 0 {
		return key, fmt.Errorf("water: (*wazero.Module).InsertFile returned invalid key")
	}

	return key, nil
}
