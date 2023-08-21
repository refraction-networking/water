package water

import (
	"errors"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/gaukas/water/internal/filesocket"
)

type WASIConfig struct {
	*wasmtime.WasiConfig

	fs filesocket.FileSocket
}

func (c *WASIConfig) SetRXfile(file *os.File) error {
	return c.SetStdinFile(file.Name())
}

func (c *WASIConfig) SetRXfd(fd uintptr) error {
	return errors.New("not implemented")
}

func (c *WASIConfig) SetTXfile(file *os.File) error {
	return c.SetStdoutFile(file.Name())
}

func (c *WASIConfig) SetTXfd(fd uintptr) error {
	return errors.New("not implemented")
}

func (c *WASIConfig) BindFileSocket(fs filesocket.FileSocket) error {
	c.fs = fs

	// RXfile is the file where received data from net.Conn is written to,
	// which will be the WrFile of the FileSocket, as it will be Write()
	// by the net.Conn.
	if err := c.SetRXfile(fs.WrFile()); err != nil {
		return err
	}

	// TXfile is the file where data should be written-to to be sent via net.Conn,
	// which will be the RdFile of the FileSocket, as it will be Read() into
	// the net.Conn.
	return c.SetTXfile(fs.RdFile())
}
