package v0

import (
	"log"
	"syscall"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
	"github.com/refraction-networking/water/watm/wasip1"
)

type dialer struct {
	wt WrappingTransport
	// dt DialingTransport
}

func (d *dialer) ConfigurableTransport() ConfigurableTransport {
	if d.wt != nil {
		if wt, ok := d.wt.(ConfigurableTransport); ok {
			return wt
		}
	}

	// if d.dt != nil {
	// 	if dt, ok := d.dt.(ConfigurableTransport); ok {
	// 		return dt
	// 	}
	// }

	return nil
}

func (d *dialer) Initialize() {
	// TODO: allow initialization on dialer
}

var d dialer

// BuildDialerWithWrappingTransport arms the dialer with a
// [WrappingTransport] that is used to wrap a [v0net.Conn] into
// another [net.Conn] by providing some high-level application
// layer protocol.
//
// Mutually exclusive with [BuildDialerWithDialingTransport].
func BuildDialerWithWrappingTransport(wt WrappingTransport) {
	d.wt = wt
	// d.dt = nil
}

// BuildDialerWithDialingTransport arms the dialer with a
// [DialingTransport] that is used to dial a remote address and
// provide high-level application layer protocol over the dialed
// connection.
//
// Mutually exclusive with [BuildDialerWithWrappingTransport].
func BuildDialerWithDialingTransport(dt DialingTransport) {
	// TODO: implement BuildDialerWithDialingTransport
	// d.dt = dt
	// d.wt = nil
	panic("BuildDialerWithDialingTransport: not implemented")
}

//export _water_dial
func _water_dial(internalFd int32) (networkFd int32) {
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
		// call v0net.Dial
		rawNetworkConn, err := v0net.Dial("", "")
		if err != nil {
			log.Printf("dial: v0net.Dial: %v", err)
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
		// TODO: implement _water_dial with DialingTransport
	} else {
		return wasip1.EncodeWATERError(syscall.EPERM) // operation not permitted
	}

	workerIdentity = identity_dialer
	return networkFd
}
