package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/gaukas/water"
	_ "github.com/gaukas/water/transport/v0"
)

var (
	localAddr  = flag.String("laddr", "", "local address to listen on")
	remoteAddr = flag.String("raddr", "", "remote address to dial")
	wasmPath   = flag.String("wasm", "", "path to wasm file")
)

func main() {
	flag.Parse()

	wasm, err := os.ReadFile(*wasmPath)
	if err != nil {
		panic(fmt.Sprintf("failed to read wasm file: %v", err))
	}

	// start using W.A.T.E.R. API below this line, have fun!
	config := &water.Config{
		TransportModuleBin: wasm,
		NetworkDialerFunc:  net.Dial, // optional field, defaults to net.Dial
	}
	// configuring the standard out of the WebAssembly instance to inherit
	// from the parent process
	config.ModuleConfig().InheritStdout()
	config.ModuleConfig().InheritStderr()

	relay, err := water.NewRelayWithContext(context.Background(), config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dialer: %v", err))
	}

	err = relay.ListenAndRelayTo("tcp", *localAddr, "tcp", *remoteAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to dial: %v", err))
	}
}
