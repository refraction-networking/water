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

//go:embed wasm/multi_package.wasm
var multiPackageWasm []byte

func main() {
	ctx := context.Background()

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	mConfig := wazero.NewModuleConfig()
	mConfig = mConfig.WithStdout(os.Stdout)
	mConfig = mConfig.WithStderr(os.Stderr)

	if _, err := r.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(func(num int32) {
		fmt.Printf("Sending %d nukes...\n", num)
	}).Export("send_nuke").NewFunctionBuilder().WithFunc(func() int32 {
		fmt.Println("Canceling nukes... Launch sequence aborted.")
		return 0
	}).Export("cancel_nuke").Instantiate(ctx); err != nil {
		panic(err)
	}

	multiPackage, err := r.InstantiateWithConfig(ctx, multiPackageWasm, mConfig)
	if err != nil {
		panic(err)
	}

	if multiPackage == nil {
		panic("multiPackage is nil")
	}

	if _, err := multiPackage.ExportedFunction("whoami").Call(ctx); err != nil {
		panic(err)
	}

	if _, err := multiPackage.ExportedFunction("attack").Call(ctx); err != nil {
		panic(err)
	}

	if _, err := multiPackage.ExportedFunction("attack_max").Call(ctx); err != nil {
		panic(err)
	}

	if _, err := multiPackage.ExportedFunction("stop").Call(ctx); err != nil {
		panic(err)
	}

	// show all imports and exports
	if cm, err := r.CompileModule(ctx, multiPackageWasm); err != nil {
		panic(err)
	} else {
		fmt.Printf("imports: %v\n", cm.AllImports())
		fmt.Printf("exports: %v\n", cm.AllExports())
	}
}
