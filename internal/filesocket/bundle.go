package filesocket

import (
	"io"
	"net"
	"os"
)

// Bundle is a combination of a FileSocket and a (net).Conn.
//
// Anything received from the net.Conn will be written into the FileSocket,
// and anything received from the FileSocket will be written into the net.Conn.
type Bundle interface {
	Start()           // start handling data transfer between the net.Conn and the FileSocket
	RxFile() *os.File // the file where received data from net.Conn is written to
	TxFile() *os.File // the file where data should be written-to to be sent via net.Conn
	net.Conn          // ONLY for LocalAddr(), RemoteAddr(), SetDeadline(), SetReadDeadline(), SetWriteDeadline()
	OnClose(func())   // optional callback to be called when the Close() method is called
}

// bundle implements Bundle
type bundle struct {
	net.Conn
	fs      FileSocket
	onClose func()
}

func BundleFileSocket(conn net.Conn, fs FileSocket) Bundle {
	return &bundle{
		Conn: conn,
		fs:   fs,
	}
}

// BundleFiles creates a FileSocket from the given files, writing
// received data from net.Conn to the rxFile and send data from the txFile
// to the net.Conn.
func BundleFiles(conn net.Conn, rxFile, txFile *os.File) Bundle {
	return &bundle{
		Conn: conn,
		fs:   NewFileSocket(txFile, rxFile),
	}
}

func (b *bundle) Start() {
	go func() {
		io.Copy(b.fs, b.Conn)
		b.fs.Close()
	}()
	go func() {
		io.Copy(b.Conn, b.fs)
		b.Conn.Close()
		b.fs.Close() // TODO: is this necessary? now added just to be safe
	}()
}

func (b *bundle) RxFile() *os.File {
	return b.fs.(*fileSocket).wrFile
}

func (b *bundle) TxFile() *os.File {
	return b.fs.(*fileSocket).rdFile
}

func (b *bundle) Close() error {
	if b.onClose != nil {
		defer b.onClose()
	}
	b.fs.Close()
	return b.Conn.Close()
}

func (b *bundle) OnClose(f func()) {
	b.onClose = f
}
