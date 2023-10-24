package test

import (
	"testing"
)

func TestIntegration(t *testing.T) {
	t.Log("TestIntegration")
}

func runner(t *testing.T, desc string) {
	t.Log("runner, desc: ", desc)
}

func TestIntegrationOuterFailureModes(t *testing.T) {

	testcases := []struct {
		desc string
	}{
		// Dialer
		{"Dial error Unreachable"},
		{"Dial error Refused"},
		{"error from network-side connection"},
		{"error from caller-side connection"},
		{"caller-side cancelation"},
		{"network-side timeout"},

		// Listener
		{"connection not accepted by caller"},
	}

	for _, c := range testcases {
		t.Run(c.desc, func(t *testing.T) {
			runner(t, c.desc)
		})
	}
}
