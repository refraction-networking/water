package v1

import "net"

type CtrlPipe struct {
	net.Conn
}

// CONTROL MESSAGE
var (
	_CTRLPIPE_EXIT = []byte{0x00}
)

func (c *CtrlPipe) WriteExit() error {
	_, err := c.Conn.Write(_CTRLPIPE_EXIT)
	return err
}
