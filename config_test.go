package water_test

import (
	"crypto/rand"
	"net"
	"reflect"
	"testing"

	"github.com/gaukas/water/internal/log"

	"github.com/gaukas/water"
)

func TestConfig_Clone(t *testing.T) {
	t.Run("Nil Config", testConfigCloneNil)
	t.Run("Valid Config", testConfigCloneValid)
}

func testConfigCloneNil(t *testing.T) {
	var c *water.Config = nil
	ccloned := c.Clone()
	if ccloned != nil {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}

func testConfigCloneValid(t *testing.T) {
	c1 := water.Config{}
	v := reflect.ValueOf(&c1).Elem()

	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := v.Field(i)
		// testing/quick can't handle functions or interfaces and so
		// isn't used here.
		switch fn := typ.Field(i).Name; fn {
		case "TransportModuleBin":
			f.Set(reflect.ValueOf(make([]byte, 256)))
		case "TransportModuleConfig":
			f.Set(reflect.ValueOf(water.TransportModuleConfigFromBytes([]byte("foo"))))
		case "NetworkDialerFunc": // functions aren't deeply equal unless nil
			continue
		case "NetworkListener":
			f.Set(reflect.ValueOf(&net.TCPListener{}))
		case "ModuleConfigFactory", "RuntimeConfigFactory":
			continue
		case "OverrideLogger":
			f.Set(reflect.ValueOf(log.DefaultLogger()))
		default:
			t.Fatalf("unhandled field: %s", fn)
		}
	}
	_, err := rand.Read(c1.TransportModuleBin)
	if err != nil {
		t.Fatalf("rand.Read error: %v", err)
	}

	c2 := c1.Clone()

	if !reflect.DeepEqual(&c1, c2) {
		t.Errorf("Clone() = %v, want %v", c2, &c1)
	}
}

func TestConfig_NetworkDialerFuncOrDefault(t *testing.T) {
	t.Run("Nil NetworkDialerFunc", testConfigNetworkDialerFuncNil)
	t.Run("Valid NetworkDialerFunc", testConfigNetworkDialerFuncValid)
}

func testConfigNetworkDialerFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panicked: %v", r)
		}
	}()

	c := &water.Config{}
	dialer := c.NetworkDialerFuncOrDefault()
	dialer("tcp", "localhost:0")
}

func testConfigNetworkDialerFuncValid(t *testing.T) {
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
	_, err := dialer("tcp", "localhost:0")
	if err != nil {
		t.Errorf("NetworkDialerFuncOrDefault() error = %v, want nil", err)
	}

	if networkBuf != "tcp" || addressBuf != "localhost:0" {
		t.Errorf("NetworkDialerFuncOrDefault() = %v, want %v", &dialer, &netDialerFunc)
	}
}

func TestConfig_NetworkListenerOrPanic(t *testing.T) {
	t.Run("Nil NetworkListener", testConfigNetworkListenerNil)
	t.Run("Valid NetworkListener", testConfigNetworkListenerValid)
}

func testConfigNetworkListenerNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Did not panic")
		}
	}()

	c := &water.Config{}
	c.NetworkListenerOrPanic()
}

func testConfigNetworkListenerValid(t *testing.T) {
	c := &water.Config{
		NetworkListener: &net.TCPListener{},
	}

	l := c.NetworkListenerOrPanic()
	if l == nil {
		t.Errorf("NetworkListenerOrPanic() = %v, want %v", l, &net.TCPListener{})
	}
}
