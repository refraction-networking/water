package v0

import (
	"log"
	"syscall"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
	"github.com/refraction-networking/water/watm/wasip1"
)

type listener struct {
	wt WrappingTransport
	// lt ListeningTransport
}

func (l *listener) ConfigurableTransport() ConfigurableTransport {
	if l.wt != nil {
		if wt, ok := l.wt.(ConfigurableTransport); ok {
			return wt
		}
	}

	// if l.lt != nil {
	// 	if lt, ok := l.lt.(ConfigurableTransport); ok {
	// 		return lt
	// 	}
	// }

	return nil
}

func (l *listener) Initialize() {
	// TODO: allow initialization on listener
}

var l listener

// BuildListenerWithWrappingTransport arms the listener with a
// [WrappingTransport] that is used to wrap a [v0net.Conn] into
// another [net.Conn] by providing some high-level application
// layer protocol.
//
// Mutually exclusive with [BuildListenerWithListeningTransport].
func BuildListenerWithWrappingTransport(wt WrappingTransport) {
	l.wt = wt
	// l.lt = nil
}

// BuildListenerWithListeningTransport arms the listener with a
// [ListeningTransport] that is used to accept incoming connections
// on a local address and provide high-level application layer
// protocol over the accepted connection.
//
// Mutually exclusive with [BuildListenerWithWrappingTransport].
func BuildListenerWithListeningTransport(lt ListeningTransport) {
	// TODO: implement BuildListenerWithListeningTransport
	// l.lt = lt
	// l.wt = nil
	panic("BuildListenerWithListeningTransport: not implemented")
}

//export _water_accept
func _water_accept(internalFd int32) (networkFd int32) {
	if workerIdentity != identity_uninitialized {
		return wasip1.EncodeWATERError(syscall.EBUSY) // device or resource busy (worker already initialized)
	}

	// wrap the internalFd into a v0net.Conn
	sourceConn = v0net.RebuildTCPConn(internalFd)
	err := sourceConn.(*v0net.TCPConn).SetNonBlock(true)
	if err != nil {
		log.Printf("dial: sourceConn.SetNonblock: %v", err)
		return wasip1.EncodeWATERError(err.(syscall.Errno))
	}

	if d.wt != nil {
		var lis v0net.Listener = &v0net.TCPListener{}
		// call v0net.Listener.Accept
		rawNetworkConn, err := lis.Accept()
		if err != nil {
			log.Printf("dial: v0net.Listener.Accept: %v", err)
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}
		networkFd = rawNetworkConn.Fd()

		// Note: here we are not setting nonblocking mode on the
		// networkConn -- it depends on the WrappingTransport to
		// determine whether to set nonblocking mode or not.

		// wrap
		remoteConn, err = d.wt.Wrap(rawNetworkConn)
		if err != nil {
			log.Printf("dial: d.wt.Wrap: %v", err)
			return wasip1.EncodeWATERError(syscall.EPROTO) // protocol error
		}
		// TODO: implement _water_accept with ListeningTransport
	} else {
		return wasip1.EncodeWATERError(syscall.EPERM) // operation not permitted
	}

	workerIdentity = identity_listener
	return networkFd
}
