package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/experimental/logging"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/poll_oneoff.wasm
var pollWasm []byte

func main() {
	ctx := context.Background()
	ctx = context.WithValue(ctx, experimental.FunctionListenerFactoryKey{},
		logging.NewHostLoggingListenerFactory(os.Stderr, logging.LogScopeFilesystem|logging.LogScopePoll|logging.LogScopeSock))

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	mConfig := wazero.NewModuleConfig()
	mConfig = mConfig.WithStdout(os.Stdout)
	mConfig = mConfig.WithStderr(os.Stderr)

	poll, err := r.InstantiateWithConfig(ctx, pollWasm, mConfig)
	if err != nil {
		panic(err)
	}

	if poll == nil {
		panic("multiPackage is nil")
	}

	if results, err := poll.ExportedFunction("poll").Call(ctx, 5, 6, 7); err != nil {
		panic(err)
	} else {
		// parse result as int32
		result := api.DecodeI32(results[0])
		fmt.Printf("poll result: %d\n", result)
	}
}
