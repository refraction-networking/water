package water

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/gaukas/water/internal/log"
)

// InsertConn implements Core.
func (c *core) InsertConn(conn net.Conn) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert TCPConn before instantiation")
	}

	switch conn := conn.(type) {
	case *net.TCPConn:
		// make it non-blocking
		if err := setNonblock(conn); err != nil {
			return 0, fmt.Errorf("water: error setting non-blocking mode: %w", err)
		}

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
		// make it non-blocking
		if err := setNonblock(listener); err != nil {
			return 0, fmt.Errorf("water: error setting non-blocking mode: %w", err)
		}

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

	// make it non-blocking
	if err := setNonblock(osFile); err != nil {
		return 0, fmt.Errorf("water: error setting non-blocking mode: %w", err)
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

func setNonblock(conn syscall.Conn) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	return rawConn.Control(func(fd uintptr) {
		if err := syscall.SetNonblock(platformSpecificFd(fd), true); err != nil {
			log.Errorf("failed to set non-blocking: %v", err)
		}
	})
}
