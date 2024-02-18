//go:build !wasip1 && !wasi

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/host_import.wasm
var hostImportWasm []byte

func main() {
	ctx := context.Background()

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	mConfig := wazero.NewModuleConfig()
	// mConfig = mConfig.WithStdout(os.Stdout) // prevent confusion on whose stdout
	mConfig = mConfig.WithStderr(os.Stderr)

	if _, err := r.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(func() {
		fmt.Println("Hello from Go!")
	}).Export("hello").Instantiate(ctx); err != nil {
		panic(err)
	}

	hostImport, err := r.InstantiateWithConfig(ctx, hostImportWasm, mConfig)
	if err != nil {
		panic(err)
	}

	if hostImport == nil {
		panic("hostImport is nil")
	}
}
