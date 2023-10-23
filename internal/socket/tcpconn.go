package socket

import (
	"fmt"
	"net"
	"sync"
)

func TCPConnPair(address ...string) (c1, c2 net.Conn, err error) {
	var tcpAddr string = ":0"
	if len(address) > 0 && address[0] != "" {
		tcpAddr = address[0]
	}

	l, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}

	var wg *sync.WaitGroup = new(sync.WaitGroup)
	var goroutineErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		c2, goroutineErr = l.Accept()
	}()

	c1, err = net.Dial("tcp", l.Addr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("net.Dial returned error: %w", err)
	}
	wg.Wait()

	if goroutineErr != nil {
		return nil, nil, fmt.Errorf("l.Accept returned error: %w", goroutineErr)
	}

	if c1 == nil || c2 == nil {
		return nil, nil, fmt.Errorf("c1 or c2 is nil")
	}

	return c1, c2, l.Close()
}
