package memfs

import (
	"errors"
	"io"
	"strings"

	wasys "github.com/tetratelabs/wazero/sys"

	"github.com/tetratelabs/wazero/experimental/sys"

	"github.com/blang/vfs"
	"github.com/blang/vfs/memfs"
)

type memoryFSFile struct {
	fs   *memfs.MemFS
	fl   vfs.File
	path string

	sys.UnimplementedFile
}

func (f *memoryFSFile) Stat() (wasys.Stat_t, sys.Errno) {
	return stat(f.fs, f.path)
}

func (f *memoryFSFile) Close() sys.Errno {
	err := f.fl.Close()
	if err != nil {
		// this will never happen
		return sys.EIO
	}
	return 0
}

func (f *memoryFSFile) IsDir() (bool, sys.Errno) {
	return false, 0
}

func (f *memoryFSFile) Read(buf []byte) (n int, errno sys.Errno) {
	n, err := f.fl.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, 0
		}
		// this will never happen
		return 0, sys.EBADF
	}
	return
}

func (f *memoryFSFile) Seek(offset int64, whence int) (newOffset int64, errno sys.Errno) {
	r, err := f.fl.Seek(offset, whence)
	if err != nil {

		if strings.Contains(err.Error(), "invalid whence") {
			return 0, sys.EINVAL
		}
		if strings.Contains(err.Error(), "negative position") {
			return 0, sys.EINVAL
		}
		if strings.Contains(err.Error(), "too far") {
			// it should be POSIX EFBIG but wazero maps that to EIO
			return 0, sys.EIO
		}
		// can never happen
		return 0, sys.EINVAL
	}
	return r, 0
}

func (f *memoryFSFile) Write(buf []byte) (n int, errno sys.Errno) {
	n, err := f.fl.Write(buf)
	if err != nil {
		// it should be POSIX EFBIG but wazero maps that to EIO
		return 0, sys.EIO
	}
	return
}
