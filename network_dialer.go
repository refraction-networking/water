package water

import (
	"net"
)

type NetworkDialer interface {
	// Dial connects to the address on the named network.
	Dial(network, address string) (net.Conn, error)
}

type netDialer struct {
	// tlsDialer NetworkDialer // for a tlsDialer, Dial() function should return a *tls.Conn or its equivalent. And Handshake() should be called before returning.
}

func DefaultNetworkDialer() NetworkDialer {
	return &netDialer{
		// tlsDialer: TLSDialerWithConfig(&tls.Config{}),
	}
}

// func NetworkDialerWithTLS(tlsDialer NetworkDialer) NetworkDialer {
// 	return &netDialer{
// 		tlsDialer: tlsDialer,
// 	}
// }

func (d *netDialer) Dial(network, address string) (net.Conn, error) {
	// switch network {
	// case "tls", "tls4", "tls6":
	// 	tlsNetwork := strings.ReplaceAll(network, "tls", "tcp") // tls4 -> tcp4, etc.
	// 	return d.tlsDialer.Dial(tlsNetwork, address)
	// default:
	return net.Dial(network, address)
	// }
}

// type tlsDialer struct {
// 	tlsConfig *tls.Config
// }

// func TLSDialerWithConfig(config *tls.Config) NetworkDialer {
// 	return &tlsDialer{config.Clone()}
// }

// func (d *tlsDialer) Dial(network, address string) (net.Conn, error) {
// 	d.tlsConfig.ServerName = strings.Split(address, ":")[0] // "example.com:443" -> "example.com"
// 	tlsConn, err := tls.Dial(network, address, d.tlsConfig)
// 	if err != nil {
// 		return nil, fmt.Errorf("tls.Dial(): %w", err)
// 	}
// 	return tlsConn, nil
// }
