package v0

import (
	"bytes"
	"errors"
	"log"
	"os"
	"syscall"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
	"github.com/refraction-networking/water/watm/wasip1"
)

// Export the WATM version indicator.
//
//	gaukas: I noticed that in Rust we can export a const variable
//	but here in Go we have to export a function instead. Luckily
//	in our standard we are not checking against its type but only
//	the name.
//
//export _water_v0
func _water_v0() {}

//export _water_init
func _water_init() int32 {
	// Check if dialer/listener/relay is configurable. If so,
	// pull the config file from the host and configure them.
	dct := d.ConfigurableTransport()
	lct := l.ConfigurableTransport()
	// rct := r.ConfigurableTransport()
	if dct != nil || lct != nil /* || rct != nil */ {
		config, err := readConfig()
		if err == nil {
			if dct != nil {
				dct.Configure(config)
			}

			if lct != nil {
				lct.Configure(config)
			}

			// if rct != nil {
			// 	rct.Configure(config)
			// }
		} else if !errors.Is(err, syscall.EACCES) { // EACCES means no config file provided by the host
			return wasip1.EncodeWATERError(err.(syscall.Errno))
		}
	}

	// TODO: initialize the dialer, listener, and relay
	d.Initialize()
	l.Initialize()
	// r.Initialize()

	return 0 // ESUCCESS
}

func readConfig() (config []byte, err error) {
	fd, err := wasip1.DecodeWATERError(_import_pull_config())
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(fd), "config")
	if file == nil {
		return nil, syscall.EBADF
	}

	// read the config file
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(file)
	if err != nil {
		log.Println("readConfig: (*bytes.Buffer).ReadFrom:", err)
		return nil, syscall.EIO
	}

	config = buf.Bytes()

	// close the file
	if err := file.Close(); err != nil {
		return config, syscall.EIO
	}

	return config, nil
}

//export _water_cancel_with
func _water_cancel_with(cancelFd int32) int32 {
	cancelConn = v0net.RebuildTCPConn(cancelFd)
	if err := cancelConn.(v0net.Conn).SetNonBlock(true); err != nil {
		log.Printf("dial: cancelConn.SetNonblock: %v", err)
		return wasip1.EncodeWATERError(err.(syscall.Errno))
	}

	return 0 // ESUCCESS
}
