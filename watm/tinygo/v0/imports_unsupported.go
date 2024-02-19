//go:build !wasip1 && !wasi

package v0

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/refraction-networking/water/watm/wasip1"
)

func _import_host_defer() {
	// just do nothing, since nothing really matters if not
	// commanded by the host.
}

// emulate the behavior when no config is provided on
// the host side.
func _import_pull_config() (fd int32) {
	return wasip1.EncodeWATERError(syscall.ENOENT)
}

// emulate the behavior when no file descriptors are
// ready and the timeout expires immediately.
func poll_oneoff(in, out unsafe.Pointer, nsubscriptions uint32, nevents unsafe.Pointer) uint32 {
	// wait for a very short period to simulate the polling
	time.Sleep(50 * time.Millisecond)
	*(*uint32)(nevents) = nsubscriptions
	return 0
}
