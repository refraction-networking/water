package main

import v0 "github.com/refraction-networking/water/watm/tinygo/v0"

func init() {
	v0.BuildDialerWithWrappingTransport(&UTLSClientWrappingTransport{})
	// v0.BuildListenerWithWrappingTransport(&UTLSClientWrappingTransport{})
	// v0.BuildRelayWithWrappingTransport(&UTLSClientWrappingTransport{}, v0.RelayWrapRemote)
}

func main() {}
