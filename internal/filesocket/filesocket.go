package filesocket

import (
	"errors"
	"io"
	"os"
	"sync/atomic"
)

type FileSocket interface {
	io.ReadWriteCloser

	RdFile() *os.File // returns the file where Read() wi;; read from
	WrFile() *os.File // returns the file where Write() will write to
}

type fileSocket struct {
	rdFile *os.File // Read() reads from this file
	wrFile *os.File // Write() writes to this file, ReadFrom() reads data from a reader into this file

	closed *atomic.Bool
}

func NewFileSocket(rdFile, wrFile *os.File) FileSocket {
	return &fileSocket{rdFile, wrFile, &atomic.Bool{}}
}

func (fs *fileSocket) Read(p []byte) (n int, err error) {
	if fs.closed.Load() {
		return 0, os.ErrClosed
	}
	if fs.rdFile == nil {
		return 0, os.ErrInvalid
	}
	n, err = fs.rdFile.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		if errors.Is(err, os.ErrClosed) && fs.closed.Load() {
			err = io.EOF // when closed by caller, return EOF instead
		}
		return 0, err
	}
	return n, nil
}

func (fs *fileSocket) ReadFrom(r io.Reader) (n int64, err error) {
	if fs.closed.Load() {
		return 0, os.ErrClosed
	}
	if fs.rdFile == nil {
		return 0, os.ErrInvalid
	}
	return fs.wrFile.ReadFrom(r) // ReadFrom() could have platform-specific benefits
}

func (fs *fileSocket) Write(p []byte) (n int, err error) {
	if fs.closed.Load() {
		return 0, os.ErrClosed
	}
	if fs.wrFile == nil {
		return 0, os.ErrInvalid
	}
	return fs.wrFile.Write(p)
}

func (fs *fileSocket) Close() error {
	if !fs.closed.CompareAndSwap(false, true) {
		return os.ErrClosed
	}
	if fs.rdFile != nil {
		fs.rdFile.Close()
		os.Remove(fs.rdFile.Name())
	}
	if fs.wrFile != nil {
		fs.wrFile.Close()
		os.Remove(fs.wrFile.Name())
	}
	return nil
}

func (fs *fileSocket) RdFile() *os.File {
	return fs.rdFile
}

func (fs *fileSocket) WrFile() *os.File {
	return fs.wrFile
}
