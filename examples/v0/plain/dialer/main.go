package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/gaukas/water"
	"github.com/gaukas/water/internal/log"
	_ "github.com/gaukas/water/transport/v0"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: dialer <remote_addr>")
	}
	var remoteAddr string = os.Args[1]

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
	config.ModuleConfig().InheritStdout()
	config.ModuleConfig().InheritStderr()

	dialer, err := water.NewDialer(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dialer: %v", err))
	}

	conn, err := dialer.Dial("tcp", remoteAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to dial: %v", err))
	}
	defer conn.Close()
	// conn is a net.Conn that you are familiar with.
	// So effectively, W.A.T.E.R. API ends here and everything below
	// this line is just how you treat a net.Conn.

	handleConn("server", conn)
}

func handleConn(peer string, conn net.Conn) {
	defer conn.Close()

	log.Infof("handling connection from/to %s(%s)", peer, conn.RemoteAddr())
	chanMsgRecv := make(chan []byte, 4) // up to 4 messages in the buffer
	// start a goroutine to read data from the connection
	go func() {
		defer close(chanMsgRecv)
		buf := make([]byte, 1024) // 1 KiB
		for {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Read(buf)
			if err != nil {
				log.Warnf("read %s: error %v, tearing down connection...", peer, err)
				conn.Close()
				return
			}
			chanMsgRecv <- buf[:n]
		}
	}()

	// start a ticker for sending data every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var sendBuf []byte = make([]byte, 4) // 4 bytes per message
	for {
		select {
		case msg := <-chanMsgRecv:
			if msg == nil {
				return // connection closed
			}
			log.Infof("%s: %x\n", peer, msg)
		case <-ticker.C:
			n, err := rand.Read(sendBuf)
			if err != nil {
				log.Warnf("rand.Read: error %v, tearing down connection...", err)
				return
			}
			// print the bytes sending as hex string
			log.Infof("sending: %x\n", sendBuf[:n])

			_, err = conn.Write(sendBuf[:n])
			if err != nil {
				log.Warnf("write %s: error %v, tearing down connection...", peer, err)
				return
			}
		}
	}
}
