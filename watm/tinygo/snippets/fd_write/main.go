//go:build !wasip1 && !wasi

package main

import (
	"context"
	_ "embed"
	"log"
	"net"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/fd_write.wasm
var fdWriteWasm []byte

func main() {
	ctx := context.Background()

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	mConfig := wazero.NewModuleConfig()
	mConfig = mConfig.WithStdout(os.Stdout)
	mConfig = mConfig.WithStderr(os.Stderr)

	fdWrite, err := r.InstantiateWithConfig(ctx, fdWriteWasm, mConfig)
	if err != nil {
		panic(err)
	}

	if fdWrite == nil {
		panic("write is nil")
	}

	// create a TCP Conn pair
	lis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	var lisConn *net.TCPConn
	var lisWg sync.WaitGroup
	lisWg.Add(1)
	go func() {
		defer lisWg.Done()
		lisConn, err = lis.AcceptTCP()
		if err != nil {
			panic(err)
		}
	}()

	dialConn, err := net.DialTCP("tcp", nil, lis.Addr().(*net.TCPAddr))
	if err != nil {
		panic(err)
	}

	lisWg.Wait()

	fd, ok := fdWrite.InsertTCPConn(lisConn)
	if !ok {
		panic("failed to insert TCPConn")
	}

	// call the function
	results, err := fdWrite.ExportedFunction("hello").Call(ctx, uint64(fd))
	if err != nil {
		log.Panicln(err)
	}

	// check the result (int)
	if len(results) != 1 {
		log.Panicln("unexpected number of results")
	}
	resultsInt := api.DecodeI32(results[0])

	var rdBuf []byte = make([]byte, 64)
	n, err := dialConn.Read(rdBuf)
	if err != nil {
		panic(err)
	}

	if n != int(resultsInt) {
		log.Panicln("unexpected number of bytes written")
	}

	log.Printf("wasm: %s", rdBuf[:n])

	// write to dialConn, read by wasm
	var sendMsg = []byte("Hello, WASMorld!\n")
	n, err = dialConn.Write(sendMsg)
	if err != nil {
		panic(err)
	}

	if n != len(sendMsg) {
		log.Panicln("unexpected number of bytes written")
	}

	// call the function

	results, err = fdWrite.ExportedFunction("world").Call(ctx, uint64(fd))
	if err != nil {
		log.Panicln(err)
	}

	// check the result (int)
	if len(results) != 1 {
		log.Panicln("unexpected number of results")
	}
	resultsInt = api.DecodeI32(results[0])

	if n != int(resultsInt) {
		log.Panicln("unexpected number of bytes written")
	}
}
