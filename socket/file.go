package socket

import (
	"fmt"
	"io"
	"os"
)

type EmbedFile interface {
	File() (*os.File, error)
}

func AsFile(f any) (*os.File, error) {
	switch f := f.(type) {
	case *os.File:
		return f, nil
	// Anything implementing EmbedFile interface, including:
	// - *net.TCPConn
	// - *net.UDPConn
	// - *net.UnixConn
	// - *net.TCPListener
	// - *net.UnixListener
	case EmbedFile:
		return f.File()
	case io.ReadWriteCloser: // and also net.Conn
		unixConn, err := UnixConnWrap(f)
		if err != nil {
			return nil, err
		}
		return unixConn.File()
	default:
		return nil, fmt.Errorf("%T cannot be converted to *os.File", f)
	}
}
