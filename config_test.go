package water_test

import (
	"crypto/rand"
	"net"
	"reflect"
	"testing"

	"github.com/gaukas/water"
)

func TestConfigClone(t *testing.T) {
	c := &water.Config{
		TMBin:             make([]byte, 256),
		NetworkDialerFunc: nil, // functions aren't deeply equal unless nil
		NetworkListener:   &net.TCPListener{},
		TMConfig: water.TMConfig{
			FilePath: "/tmp/watm.toml",
		},
	}

	_, err := rand.Read(c.TMBin)
	if err != nil {
		t.Fatalf("rand.Read error: %v", err)
	}

	ccloned := c.Clone()

	if !reflect.DeepEqual(c, ccloned) {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}
