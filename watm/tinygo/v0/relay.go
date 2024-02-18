package v0

import (
	"log"
	"syscall"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
	"github.com/refraction-networking/water/watm/wasip1"
)

type RelayWrapSelection bool

const (
	RelayWrapRemote RelayWrapSelection = false
	RelayWrapSource RelayWrapSelection = true
)

type relay struct {
	wt            WrappingTransport
	wrapSelection RelayWrapSelection
	// lt ListeningTransport
	// dt DialingTransport
}

func (r *relay) ConfigurableTransport() ConfigurableTransport {
	if r.wt != nil {
		if wt, ok := r.wt.(ConfigurableTransport); ok {
			return wt
		}
	}

	// if r.lt != nil {
	// 	if lt, ok := r.lt.(ConfigurableTransport); ok {
	// 		return lt
	// 	}
	// }

	// if r.dt != nil {
	// 	if dt, ok := r.dt.(ConfigurableTransport); ok {
	// 		return dt
	// 	}
	// }

	return nil
}

func (r *relay) Initialize() {
	// TODO: allow initialization on relay
}

var r relay

// BuildRelayWithWrappingTransport arms the relay with a
// [WrappingTransport] that is used to wrap a [v0net.Conn] into
// another [net.Conn] by providing some high-level application
// layer protocol.
//
// The caller MUST keep in mind that the [WrappingTransport] is
// used to wrap the connection to the remote address, not the
// connection from the source address (the dialing peer).
// To reverse this behavior, i.e., wrap the inbounding connection,
// set wrapSelection to [RelayWrapSource].
//
// Mutually exclusive with [BuildRelayWithListeningDialingTransport].
func BuildRelayWithWrappingTransport(wt WrappingTransport, wrapSelection RelayWrapSelection) {
	r.wt = wt
	r.wrapSelection = wrapSelection
	// r.lt = nil
	// r.dt = nil
}

// BuildRelayWithListeningDialingTransport arms the relay with a
// [ListeningTransport] that is used to accept incoming connections
// on a local address and provide high-level application layer
// protocol over the accepted connection, and a [DialingTransport]
// that is used to dial a remote address and provide high-level
// application layer protocol over the dialed connection.
//
// Mutually exclusive with [BuildRelayWithWrappingTransport].
func BuildRelayWithListeningDialingTransport(lt ListeningTransport, dt DialingTransport) {
	// TODO: implement BuildRelayWithListeningDialingTransport
	// r.lt = lt
	// r.dt = dt
	// r.wt = nil
	panic("BuildRelayWithListeningDialingTransport: not implemented")
}

//export _water_associate
func _water_associate() int32 {
	if workerIdentity != identity_uninitialized {
		return wasip1.EncodeWATERError(syscall.EBUSY) // device or resource busy (worker already initialized)
	}

	if r.wt != nil {
		var err error
		var lis v0net.Listener = &v0net.TCPListener{}
		sourceConn, err = lis.Accept()
		if err != nil {
			log.Printf("dial: v0net.Listener.Accept: %v", err)
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}

		remoteConn, err = v0net.Dial("", "")
		if err != nil {
			log.Printf("dial: v0net.Dial: %v", err)
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}

		if r.wrapSelection == RelayWrapRemote {
			// wrap remoteConn
			remoteConn, err = r.wt.Wrap(remoteConn.(*v0net.TCPConn))
			// set sourceConn, the not-wrapped one, to non-blocking mode
			sourceConn.(*v0net.TCPConn).SetNonBlock(true)
		} else {
			// wrap sourceConn
			sourceConn, err = r.wt.Wrap(sourceConn.(*v0net.TCPConn))
			// set remoteConn, the not-wrapped one, to non-blocking mode
			remoteConn.(*v0net.TCPConn).SetNonBlock(true)
		}
		if err != nil {
			log.Printf("dial: r.wt.Wrap: %v", err)
			return wasip1.EncodeWATERError(syscall.EPROTO) // protocol error
		}
	} else {
		return wasip1.EncodeWATERError(syscall.EPERM) // operation not permitted
	}

	workerIdentity = identity_relay
	return 0
}
