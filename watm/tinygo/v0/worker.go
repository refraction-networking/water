package v0

import (
	"errors"
	"io"
	"log"
	"net"
	"syscall"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
	"github.com/refraction-networking/water/watm/wasip1"
)

type identity uint8

var workerIdentity identity = identity_uninitialized

const (
	identity_uninitialized identity = iota
	identity_dialer
	identity_listener
	identity_relay
)

var identityStrings = map[identity]string{
	identity_dialer:   "dialer",
	identity_listener: "listener",
	identity_relay:    "relay",
}

var sourceConn v0net.Conn // sourceConn is used to communicate between WASM and the host application or a dialing party (for relay only)
var remoteConn v0net.Conn // remoteConn is used to communicate between WASM and a dialed remote destination (for dialer/relay) or a dialing party (for listener only)
var cancelConn v0net.Conn // cancelConn is used to cancel the entire worker.

var workerFn func() int32 = unfairWorker // by default, use unfairWorker

var readBuf []byte = make([]byte, 16384) // 16k buffer for reading

// WorkerFairness sets the fairness of a worker.
//
// If sourceConn or remoteConn will not work in non-blocking mode,
// it is highly recommended to set fair to true, otherwise it is most
// likely that the worker will block on reading from a blocking
// connection forever and therefore make no progress in the other
// direction.
func WorkerFairness(fair bool) {
	if fair {
		workerFn = fairWorker
	} else {
		workerFn = unfairWorker
	}
}

//export _water_worker
func _water_worker() int32 {
	if workerIdentity == identity_uninitialized {
		log.Println("worker: uninitialized")
		return wasip1.EncodeWATERError(syscall.ENOTCONN) // socket not connected
	}
	log.Printf("worker: working as %s", identityStrings[workerIdentity])
	return worker()
}

func worker() int32 {
	defer _import_host_defer()

	if sourceConn == nil || remoteConn == nil || cancelConn == nil {
		log.Println("worker: worker: sourceConn, remoteConn, or cancelConn is nil")
		return wasip1.EncodeWATERError(syscall.EBADF) // bad file descriptor
	}

	return workerFn()
}

// untilError executes the given function until non-nil error is returned
func untilError(f func() error) error {
	var err error
	for err == nil {
		err = f()
	}
	return err
}

// unfairWorker works on all three connections with a priority order
// of cancelConn > sourceConn > remoteConn.
//
// It keeps working on the current connection until it returns an error,
// and if the error is EAGAIN, it switches to the next connection. If the
// connection is not properly set to non-blocking mode, i.e., never returns
// EAGAIN, this function will block forever and never work on a lower priority
// connection. Thus it is called unfairWorker.
//
// TODO: use poll_oneoff instead of busy polling
func unfairWorker() int32 {
	for {
		pollFd := []pollFd{
			{
				fd:     uintptr(cancelConn.Fd()),
				events: EventFdRead,
			},
			{
				fd:     uintptr(sourceConn.Fd()),
				events: EventFdRead,
			},
			{
				fd:     uintptr(remoteConn.Fd()),
				events: EventFdRead,
			},
		}

		n, err := _poll(pollFd, -1)
		if n == 0 { // TODO: re-evaluate the condition
			if err == nil || errors.Is(err, syscall.EAGAIN) {
				usleep(100) // wait 100us before retrying _poll
				continue
			}
			log.Println("worker: unfairWorker: _poll:", err)
			return int32(err.(syscall.Errno))
		}

		// 1st priority: cancelConn
		_, err = cancelConn.Read(readBuf)
		if !errors.Is(err, syscall.EAGAIN) {
			if errors.Is(err, io.EOF) || err == nil {
				log.Println("worker: unfairWorker: cancelConn is closed")
				return wasip1.EncodeWATERError(syscall.ECANCELED) // operation canceled
			}
			log.Println("worker: unfairWorker: cancelConn.Read:", err)
			return wasip1.EncodeWATERError(syscall.EIO) // input/output error
		}

		// 2nd priority: sourceConn
		if err := untilError(func() error {
			readN, readErr := sourceConn.Read(readBuf)
			if readErr != nil {
				return readErr
			}

			writeN, writeErr := remoteConn.Write(readBuf[:readN])
			if writeErr != nil {
				log.Println("worker: unfairWorker: remoteConn.Write:", writeErr)
				return syscall.EIO // input/output error, we cannot retry async write yet
			}

			if readN != writeN {
				log.Println("worker: unfairWorker: readN != writeN")
				return syscall.EIO // input/output error
			}

			return nil
		}); !errors.Is(err, syscall.EAGAIN) {
			if errors.Is(err, io.EOF) {
				log.Println("worker: unfairWorker: sourceConn is closed")
				return wasip1.EncodeWATERError(syscall.EPIPE) // broken pipe
			}
			log.Println("worker: unfairWorker: sourceConn.Read:", err)
			return wasip1.EncodeWATERError(syscall.EIO) // input/output error
		}

		// 3rd priority: remoteConn
		if err := untilError(func() error {
			readN, readErr := remoteConn.Read(readBuf)
			if readErr != nil {
				return readErr
			}

			writeN, writeErr := sourceConn.Write(readBuf[:readN])
			if writeErr != nil {
				log.Println("worker: unfairWorker: sourceConn.Write:", writeErr)
				return syscall.EIO // input/output error, we cannot retry async write yet
			}

			if readN != writeN {
				log.Println("worker: unfairWorker: readN != writeN")
				return syscall.EIO // input/output error
			}

			return nil
		}); !errors.Is(err, syscall.EAGAIN) {
			if errors.Is(err, io.EOF) {
				log.Println("worker: unfairWorker: remoteConn is closed")
				return wasip1.EncodeWATERError(syscall.EPIPE) // broken pipe
			}
			log.Println("worker: unfairWorker: remoteConn.Read:", err)
			return wasip1.EncodeWATERError(syscall.EIO) // input/output error
		}
	}
}

// like unfairWorker, fairWorker also works on all three connections with a priority order
// of cancelConn > sourceConn > remoteConn.
//
// But different from unfairWorker, fairWorker spend equal amount of turns on each connection
// for calling Read. Therefore it has a better fairness than unfairWorker, which may still
// make progress if one of the connection is not properly set to non-blocking mode.
//
// TODO: use poll_oneoff instead of busy polling
func fairWorker() int32 {
	for {
		pollFd := []pollFd{
			{
				fd:     uintptr(cancelConn.Fd()),
				events: EventFdRead,
			},
			{
				fd:     uintptr(sourceConn.Fd()),
				events: EventFdRead,
			},
			{
				fd:     uintptr(remoteConn.Fd()),
				events: EventFdRead,
			},
		}

		n, err := _poll(pollFd, -1)
		if n == 0 { // TODO: re-evaluate the condition
			if err == nil || errors.Is(err, syscall.EAGAIN) {
				usleep(100) // wait 100us before retrying _poll
				continue
			}
			log.Println("worker: unfairWorker: _poll:", err)
			return int32(err.(syscall.Errno))
		}

		// 1st priority: cancelConn
		_, err = cancelConn.Read(readBuf)
		if !errors.Is(err, syscall.EAGAIN) {
			if errors.Is(err, io.EOF) || err == nil {
				log.Println("worker: unfairWorker: cancelConn is closed")
				return wasip1.EncodeWATERError(syscall.ECANCELED) // operation canceled
			}
			log.Println("worker: unfairWorker: cancelConn.Read:", err)
			return wasip1.EncodeWATERError(syscall.EIO) // input/output error
		}

		// 2nd priority: sourceConn -> remoteConn
		if err := copyOnce(
			"remoteConn", // dstName
			"sourceConn", // srcName
			remoteConn,   // dst
			sourceConn,   // src
			readBuf); err != nil {
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}

		// 3rd priority: remoteConn -> sourceConn
		if err := copyOnce(
			"sourceConn", // dstName
			"remoteConn", // srcName
			sourceConn,   // dst
			remoteConn,   // src
			readBuf); err != nil {
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}
	}
}

func copyOnce(dstName, srcName string, dst, src net.Conn, buf []byte) error {
	if len(buf) == 0 {
		buf = make([]byte, 16384) // 16k buffer for reading
	}

	readN, readErr := src.Read(buf)
	if !errors.Is(readErr, syscall.EAGAIN) { // if EAGAIN, do nothing and return
		if errors.Is(readErr, io.EOF) {
			return syscall.EPIPE // broken pipe
		} else if readErr != nil {
			log.Printf("worker: copyOnce: %s.Read: %v", srcName, readErr)
			return syscall.EIO // input/output error
		}

		writeN, writeErr := dst.Write(buf[:readN])
		if writeErr != nil {
			log.Printf("worker: copyOnce: %s.Write: %v", dstName, writeErr)
			return syscall.EIO // no matter input/output error or EAGAIN we cannot retry async write yet
		}

		if readN != writeN {
			log.Printf("worker: copyOnce: %s.read != %s.written", srcName, dstName)
			return syscall.EIO // input/output error
		}
	}

	return nil
}
