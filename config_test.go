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

	rand.Read(c.TMBin)

	ccloned := c.Clone()

	if !reflect.DeepEqual(c, ccloned) {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}
