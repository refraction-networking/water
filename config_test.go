package water_test

import (
	"crypto/rand"
	"net"
	"reflect"
	"testing"

	"github.com/gaukas/water"
)

func TestConfig(t *testing.T) {
	t.Run("Clone", testConfigClone)
	t.Run("NetworkDialerFuncOrDefault", testConfigNetworkDialerFuncOrDefault)
}

func testConfigClone(t *testing.T) {
	t.Run("Config is nil", testConfigCloneNil)
	t.Run("Config is valid", testConfigCloneNonNil)
}

func testConfigCloneNil(t *testing.T) {
	var c *water.Config
	ccloned := c.Clone()
	if ccloned != nil {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}

func testConfigCloneNonNil(t *testing.T) {
	c := &water.Config{
		TransportModuleBin:    make([]byte, 256),
		NetworkDialerFunc:     nil, // functions aren't deeply equal unless nil
		NetworkListener:       &net.TCPListener{},
		TransportModuleConfig: water.TransportModuleConfigFromBytes([]byte("foo")),
	}

	_, err := rand.Read(c.TransportModuleBin)
	if err != nil {
		t.Fatalf("rand.Read error: %v", err)
	}

	ccloned := c.Clone()

	if !reflect.DeepEqual(c, ccloned) {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}

func testConfigNetworkDialerFuncOrDefault(t *testing.T) {
	t.Run("NetworkDialerFunc is not nil", testConfigNetworkDialerFuncNotNil)
}

func testConfigNetworkDialerFuncNotNil(t *testing.T) {
	var networkBuf, addressBuf string
	var netDialerFunc func(network, address string) (net.Conn, error) = func(network, address string) (net.Conn, error) {
		networkBuf = network
		addressBuf = address
		return nil, nil
	}

	c := &water.Config{
		NetworkDialerFunc: netDialerFunc,
	}

	dialer := c.NetworkDialerFuncOrDefault()
	dialer("tcp", "localhost:0")

	if networkBuf != "tcp" || addressBuf != "localhost:0" {
		t.Errorf("NetworkDialerFuncOrDefault() = %v, want %v", &dialer, &netDialerFunc)
	}
}
