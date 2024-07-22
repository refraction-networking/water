package pipe

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
)

// UnixPipe creates a pair of interconnected [net.UnixConn]. Data written
// to one connection will become readable from the other.
func UnixPipe(listenAddr *net.UnixAddr) (c1, c2 *net.UnixConn, err error) {
	if listenAddr == nil {
		// randomize a socket name
		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			return nil, nil, fmt.Errorf("crypto/rand.Read returned error: %w", err)
		}

		unixPath := os.TempDir() + string(os.PathSeparator) + fmt.Sprintf("%x", randBytes)
		if listenAddr, err = net.ResolveUnixAddr("unix", unixPath); err != nil {
			return nil, nil, fmt.Errorf("net.ResolveUnixAddr returned error: %w", err)
		}
	}

	// Temporary UnixListener
	ul, err := net.ListenUnix("unix", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}

	if c1, err = net.DialUnix("unix", nil, listenAddr); err != nil {
		return nil, nil, fmt.Errorf("net.Dial returned error: %w", err)
	}

	if c2, err = ul.AcceptUnix(); err != nil {
		return nil, nil, fmt.Errorf("(*net.UnixListener).Accept returned error: %w", err)
	}

	if c1 == nil || c2 == nil {
		return nil, nil, fmt.Errorf("unexpected nil connection without error")
	}

	if err := ul.Close(); err != nil {
		return c1, c2, fmt.Errorf("ul.Close() failed: %w", err) // this error is not fatal
	}
	return c1, c2, nil
}
