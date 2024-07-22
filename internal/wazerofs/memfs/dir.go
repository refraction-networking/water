package memfs

import (
	wasys "github.com/tetratelabs/wazero/sys"

	"github.com/tetratelabs/wazero/experimental/sys"

	"github.com/blang/vfs/memfs"
)

type memoryFSDir struct {
	fs   *memfs.MemFS
	path string

	sys.UnimplementedFile
}

func (f *memoryFSDir) IsDir() (bool, sys.Errno) {
	return true, 0
}

func (f *memoryFSDir) Stat() (wasys.Stat_t, sys.Errno) {
	return stat(f.fs, f.path)
}
