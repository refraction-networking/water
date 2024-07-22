package pipe

import (
	"fmt"
	"net"
)

// TCPPipe creates a pair of interconnected [net.TCPConn]. Data written
// to one connection will become readable from the other.
func TCPPipe(listenAddr *net.TCPAddr) (c1, c2 *net.TCPConn, err error) {
	if listenAddr == nil {
		listenAddr, err = net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return nil, nil, fmt.Errorf("net.ResolveTCPAddr returned error: %w", err)
		}
	}

	// Temporary TCPListener
	l, err := net.ListenTCP("tcp", listenAddr) // skipcq: GSC-G102
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}

	if c1, err = net.DialTCP("tcp", nil, l.Addr().(*net.TCPAddr)); err != nil {
		return nil, nil, fmt.Errorf("net.Dial returned error: %w", err)
	}

	if c2, err = l.AcceptTCP(); err != nil {
		return nil, nil, fmt.Errorf("(*net.TCPListener).Accept returned error: %w", err)
	}

	if c1 == nil || c2 == nil {
		return nil, nil, fmt.Errorf("unexpected nil connection without error")
	}

	if err := l.Close(); err != nil {
		return c1, c2, fmt.Errorf("l.Close() failed: %w", err) // this error is not fatal
	}
	return c1, c2, nil
}
