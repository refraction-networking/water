//go:build !wasip1 && !wasi

package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/file_open.wasm
var fileOpenWasm []byte

func main() {
	// create a local temp file
	file, err := os.CreateTemp("", "file_open")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	defer os.Remove(file.Name())

	ctx := context.Background()

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	fsConfig := wazero.NewFSConfig()
	fsConfig = fsConfig.WithDirMount(os.TempDir(), "/tmp")

	mConfig := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithFSConfig(fsConfig).
		WithArgs(
			"/tmp/"+file.Name()[len(os.TempDir()):],
			"/tmp/"+file.Name()[len(os.TempDir()):]+"unexist",
		)

	fileOpen, err := r.InstantiateWithConfig(ctx, fileOpenWasm, mConfig)
	if err != nil {
		panic(err)
	}

	if fileOpen == nil {
		panic("fileOpen is nil")
	}

	// WASM open an existing file
	_, err = fileOpen.ExportedFunction("open_exist_file").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}

	// read the file
	data, err := os.ReadFile(file.Name())
	if err != nil {
		panic(err)
	}

	if string(data) != "Hello, World!\n" {
		panic("data != Hello, World!\n")
	} else {
		log.Printf("content of %s: Hello, World!\n", file.Name())
	}

	// WASM open(create) a new file
	_, err = fileOpen.ExportedFunction("open_new_file").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}

	// read the file
	data, err = os.ReadFile(file.Name() + "unexist")
	if err != nil {
		panic(err)
	}
	defer os.Remove(file.Name() + "unexist")

	if string(data) != "Hello, Void!\n" {
		panic("data != Hello, Void!\n")
	} else {
		log.Printf("content of %s: Hello, Void!\n", file.Name()+"unexist")
	}

	// WASM open a file descriptor
	tmpFile, err := os.CreateTemp("", "fd_open")
	if err != nil {
		panic(err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if fd, ok := fileOpen.InsertOSFile(tmpFile); ok {
		_, err = fileOpen.ExportedFunction("open_fd").Call(ctx, uint64(fd))
		if err != nil {
			log.Panicln(err)
		}
	} else {
		panic("fd is nil")
	}

	// read the file
	data, err = os.ReadFile(tmpFile.Name())
	if err != nil {
		panic(err)
	}

	if string(data) != "Hello, FD!\n" {
		panic("data != Hello, FD!\n")
	} else {
		log.Printf("content of %s: Hello, FD!\n", tmpFile.Name())
	}

	// WASM open a file descriptor that does not exist

	if _, err = fileOpen.ExportedFunction("open_bad_fd").Call(ctx, uint64(20)); err != nil {
		log.Panicln(err)
	}

	time.Sleep(2 * time.Second)
}
