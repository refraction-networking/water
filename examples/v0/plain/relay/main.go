package main

import (
	"fmt"
	"net"
	"os"

	"github.com/gaukas/water"
	_ "github.com/gaukas/water/transport/v0"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: relay <local_addr> <remote_addr>")
	}
	var localAddr string = os.Args[1]
	var remoteAddr string = os.Args[2]

	wasm, err := os.ReadFile("./examples/v0/plain/plain.wasm")
	if err != nil {
		panic(fmt.Sprintf("failed to read wasm file: %v", err))
	}

	// start using W.A.T.E.R. API below this line, have fun!
	config := &water.Config{
		TMBin:             wasm,
		NetworkDialerFunc: net.Dial, // optional field, defaults to net.Dial
	}
	// configuring the standard out of the WebAssembly instance to inherit
	// from the parent process
	config.WASIConfig().InheritStdout()
	config.WASIConfig().InheritStderr()

	relay, err := water.NewRelay(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dialer: %v", err))
	}

	err = relay.ListenAndRelayTo("tcp", localAddr, "tcp", remoteAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to dial: %v", err))
	}
}
