package test

import (
	"testing"
)

func R(t *testing.T, desc string) {
	t.Log("runner, desc: ", desc)
}

func TestIntegrationInnerFailureModes(t *testing.T) {

	testcases := []struct {
		desc string
	}{
		// Dialer
		{"clean tunnel close"},
		{"error tunnel close"},
		{"encoder or decoder thread crashes"},
		{"handshake failure"},
		{"encode / decode error"},
		{"other wasm panic"},
		{"thread stuck on blocking read / write"},
	}

	for _, c := range testcases {
		t.Run(c.desc, func(t *testing.T) {
			R(t, c.desc)
		})
	}
}
