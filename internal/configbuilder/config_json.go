package configbuilder

// ConfigJSON defines the JSON format of the Config.
//
// This struct may fail to fully represent the Config struct, as it is
// non-trivial to represent a func or other non-serialized structures.
type ConfigJSON struct {
	TransportModule struct {
		Bin    []byte `json:"bin"`              // Base64 encoded .wasm binary
		Config []byte `json:"config,omitempty"` // Base64 encoded WATM config file content
	} `json:"transport_module"`

	Network struct {
		// DialerFunc string `json:"dialer_func,omitempty"` // we have no good way to represent a func in JSON format yet
		Listener struct {
			Network string `json:"network"` // e.g. "tcp"
			Address string `json:"address"` // e.g. "0.0.0.0:0"
		} `json:"listener,omitempty"`
	} `json:"network,omitempty"`

	Module struct {
		Argv          []string          `json:"argv,omitempty"`
		Env           map[string]string `json:"env,omitempty"`
		InheritStdin  bool              `json:"inherit_stdin,omitempty"`
		InheritStdout bool              `json:"inherit_stdout,omitempty"`
		InheritStderr bool              `json:"inherit_stderr,omitempty"`
		PreopenedDir  map[string]string `json:"preopened_dir,omitempty"` // hostPath: guestPath
	} `json:"module,omitempty"`
}
