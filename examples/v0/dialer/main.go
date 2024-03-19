package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/refraction-networking/water"
	"github.com/refraction-networking/water/internal/log"
	_ "github.com/refraction-networking/water/transport/v0"
)

var (
	remoteAddr = flag.String("raddr", "", "remote address to dial")
	wasmPath   = flag.String("wasm", "", "path to wasm file")
	remoteConn net.Conn
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

	ctx := context.Background()
	// // optional: enable wazero logging
	// ctx = context.WithValue(ctx, experimental.FunctionListenerFactoryKey{},
	// 	logging.NewHostLoggingListenerFactory(os.Stderr, logging.LogScopeFilesystem|logging.LogScopePoll|logging.LogScopeSock))

	dialer, err := water.NewDialerWithContext(ctx, config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dialer: %v", err))
	}

	conn, err := dialer.DialContext(ctx, "tcp", *remoteAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to dial: %v", err))
	}
	defer conn.Close()
	// conn is a net.Conn that you are familiar with.
	// So effectively, W.A.T.E.R. API ends here and everything below
	// this line is just how you treat a net.Conn.

	remoteConn = conn

	worker()
}

func worker() {
	defer remoteConn.Close()

	log.Infof("Connected to %s", remoteConn.RemoteAddr())
	chanMsgRecv := make(chan []byte, 4) // up to 4 messages in the buffer
	// start a goroutine to read data from the connection
	go func() {
		defer close(chanMsgRecv)
		buf := make([]byte, 1024) // 1 KiB
		for {
			n, err := remoteConn.Read(buf)
			if err != nil {
				log.Warnf("read remoteConn: error %v, tearing down connection...", err)
				remoteConn.Close()
				return
			}
			chanMsgRecv <- buf[:n]
		}
	}()

	// start a ticker for sending message every 5 seconds
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var memStats runtime.MemStats

	var sendBuf []byte = make([]byte, 4) // 4 bytes per message
	for {
		select {
		case msg := <-chanMsgRecv:
			if msg == nil {
				return // connection closed
			}
			log.Infof("peer: %x\n", msg)
		case <-ticker.C:
			n, err := rand.Read(sendBuf)
			if err != nil {
				log.Warnf("rand.Read: error %v, tearing down connection...", err)
				return
			}
			// print the bytes sending as hex string
			log.Infof("sending: %x\n", sendBuf[:n])

			_, err = remoteConn.Write(sendBuf[:n])
			if err != nil {
				log.Warnf("write: error %v, tearing down connection...", err)
				return
			}
			runtime.ReadMemStats(&memStats)

			log.Infof("Alloc: %dMB, TotalAlloc: %dMB, Sys: %dMB, NumGC: %d\n",
				memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)
		}
	}
}
