package water

type Config struct {
	Dialer   Dialer  // A dialer supporting multiple network types.
	Feature  Feature // Bit-masked experimental features
	WABin    []byte  // WebAssembly module binary.
	WAConfig []byte  // WebAssembly module config file, if any.
}

// complete the config by filling in the missing optional fields
// with fault values and panic if any of the required fields are not
// provided.
func (c *Config) init() {
	if c.Dialer == nil {
		c.Dialer = DefaultDialer()
	}

	if len(c.WABin) == 0 {
		panic("water: WASI binary is not provided")
	}
}
