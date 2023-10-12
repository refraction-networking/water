package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/gaukas/water"
	"github.com/gaukas/water/internal/log"

	waterconfig "github.com/gaukas/water/config"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: client <remote addr>")
	}
	remoteAddr := os.Args[1]

	// read file into hexencoder_v0
	hexencoder_v0, err := os.ReadFile("./examples/hexencoder/hexencoder_v0.wasm")
	if err != nil {
		panic(err)
	}

	// Dial
	dialer := &water.Dialer{
		Config: &waterconfig.Config{
			WATMBin: hexencoder_v0,
			WATMConfig: waterconfig.WATMConfig{
				FilePath: "./examples/hexencoder/hexencoder_v0.dialer.json",
			},
		},
	}
	dialer.Config.WASIConfig().InheritStdout()

	rConn, err := dialer.Dial("tcp", remoteAddr)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	// simulate Go GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// spin up two goroutines to read and write
	// go func() {
	// 	for {
	// 		buf := make([]byte, 1024)
	// 		n, err := rConn.Read(buf)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		log.Infof("Server: %s", buf[:n])
	// 		// time.Sleep(5 * time.Second)
	// 	}
	// }()

	go func() {
		var cntr int
		for {
			msg := fmt.Sprintf("hello world (%d)", cntr)
			_, err := rConn.Write([]byte(msg))
			if err != nil {
				panic(err)
			}
			cntr++
			log.Infof("Client: %s", msg)
			time.Sleep(5 * time.Second)
		}
	}()

	// wait forever
	select {}
}
