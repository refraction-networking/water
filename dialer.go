package water

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
)

type Dialer interface {
	// Dial connects to the address on the named network.
	Dial(network, address string) (net.Conn, error)
}

type dialer struct {
	tlsDialer Dialer // for a tlsDialer, Dial() function should return a *tls.Conn or its equivalent. And Handshake() should be called before returning.
}

func DefaultDialer() Dialer {
	return &dialer{
		tlsDialer: TLSDialerWithConfig(&tls.Config{}),
	}
}

func DialerWithTLS(tlsDialer Dialer) Dialer {
	return &dialer{
		tlsDialer: tlsDialer,
	}
}

func (d *dialer) Dial(network, address string) (net.Conn, error) {
	switch network {
	case "tls", "tls4", "tls6":
		tlsNetwork := strings.ReplaceAll(network, "tls", "tcp") // tls4 -> tcp4, etc.
		return d.tlsDialer.Dial(tlsNetwork, address)
	default:
		return net.Dial(network, address)
	}
}

type tlsDialer struct {
	tlsConfig *tls.Config
}

func TLSDialerWithConfig(config *tls.Config) Dialer {
	return &tlsDialer{config.Clone()}
}

func (d *tlsDialer) Dial(network, address string) (net.Conn, error) {
	d.tlsConfig.ServerName = strings.Split(address, ":")[0] // "example.com:443" -> "example.com"
	tlsConn, err := tls.Dial(network, address, d.tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("tls.Dial(): %w", err)
	}
	return tlsConn, nil
}

type AddressedDialer interface {
	Dial(network string) (net.Conn, error)
}

type addressedDialer struct {
	dialer  Dialer
	address string
}

func SetDialerAddress(dialer Dialer, address string) AddressedDialer {
	return &addressedDialer{
		dialer:  dialer,
		address: address,
	}
}

func (d *addressedDialer) Dial(network string) (net.Conn, error) {
	return d.dialer.Dial(network, d.address)
}
