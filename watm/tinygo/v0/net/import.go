//go:build !wasip1 && !wasi

package net

var hostDialedFD int32 = -1

func SetHostDialedFD(fd int32) {
	hostDialedFD = fd
}

// This function should be imported from the host in WASI.
// On non-WASI platforms, it mimicks the behavior of the host
// by returning a file descriptor of preset value.
func _import_host_dial() (fd int32) {
	return hostDialedFD
}

var hostAcceptedFD int32 = -1

func SetHostAcceptedFD(fd int32) {
	hostAcceptedFD = fd
}

// This function should be imported from the host in WASI.
// On non-WASI platforms, it mimicks the behavior of the host
// by returning a file descriptor of preset value.
func _import_host_accept() (fd int32) {
	return hostAcceptedFD
}
