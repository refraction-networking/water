package config

import (
	"crypto/rand"
	"net"
	"reflect"
	"testing"
)

func TestConfigClone(t *testing.T) {
	c := &Config{
		TMBin:           make([]byte, 256),
		DialerFunc:      nil, // functions aren't deeply equal unless nil
		NetworkListener: &net.TCPListener{},
		TMConfig: TMConfig{
			FilePath: "/tmp/watm.toml",
		},
	}

	rand.Read(c.TMBin)

	ccloned := c.Clone()

	if !reflect.DeepEqual(c, ccloned) {
		t.Errorf("Clone() = %v, want %v", ccloned, c)
	}
}
