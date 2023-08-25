package water

type Config struct {
	// WASI contains the compiled WASI binary in bytes.
	WASI []byte

	// Dialer is used to dial a network connection.
	Dialer Dialer
}

// init() checks if the Config is valid and initializes
// the Config with default values if optional fields are not provided.
func (c *Config) init() {
	if len(c.WASI) == 0 {
		panic("water: WASI binary is not provided")
	}

	if c.Dialer == nil {
		c.Dialer = DefaultDialer()
	}
}
