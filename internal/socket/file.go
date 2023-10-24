package socket

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gaukas/water/internal/log"
)

// ErrNoKnownConversion is returned when the given object cannot be converted to *os.File
var ErrNoKnownConversion = errors.New("no known conversion to *os.File")

// EmbedFile is an interface for objects that can be converted to *os.File
type EmbedFile interface {
	File() (*os.File, error)
}

// AsFile converts the given object to *os.File
func AsFile(f any) (*os.File, error) {
	switch f := f.(type) {
	case *os.File:
		log.Debugf("%T is already *os.File", f)
		return f, nil
	// Anything implementing EmbedFile interface, including:
	// - *net.TCPConn
	// - *net.UDPConn
	// - *net.UnixConn
	// - *net.TCPListener
	// - *net.UnixListener
	case EmbedFile:
		log.Debugf("%T has implemented File() (*os.File, error)", f)
		return f.File()
	case io.ReadWriteCloser: // and also net.Conn
		log.Debugf("%T implements only ReadWriteCloser and needs wrapping", f)
		unixConn, err := UnixConnWrap(f)
		if err != nil {
			return nil, err
		}
		return unixConn.File()
	default:
		log.Debugf("%T has no known conversion to *os.File", f)
		return nil, fmt.Errorf("%T: %w", f, ErrNoKnownConversion)
	}
}
