package configbuilder

// ConfigJSON defines the JSON format of the Config.
//
// This struct may fail to fully represent the Config struct, as it is
// non-trivial to represent a func or other non-serialized structures.
type ConfigJSON struct {
	TransportModule struct {
		BinPath    string `json:"bin"`              // Path to the transport module binary
		ConfigPath string `json:"config,omitempty"` // Path to the transport module config file
	} `json:"transport_module"`

	Network struct {
		// DialerFunc string `json:"dialer_func,omitempty"` // we have no good way to represent a func in JSON format yet
		Listener struct {
			Network string `json:"network"` // e.g. "tcp"
			Address string `json:"address"` // e.g. "0.0.0.0:0"
		} `json:"listener,omitempty"`
	} `json:"network,omitempty"`

	Module struct {
		Argv          []string          `json:"argv,omitempty"` // Warning: this isn't a recommended way to pass configuration to the WebAssembly module. Instead, use TransportModuleConfig for a serializable configuration file.
		Env           map[string]string `json:"env,omitempty"`  // Warning: this isn't a recommended way to pass configuration to the WebAssembly module. Instead, use TransportModuleConfig for a serializable configuration file.
		InheritStdin  bool              `json:"inherit_stdin,omitempty"`
		InheritStdout bool              `json:"inherit_stdout,omitempty"`
		InheritStderr bool              `json:"inherit_stderr,omitempty"`
		PreopenedDirs map[string]string `json:"preopened_dirs,omitempty"` // hostPath: guestPath
	} `json:"module,omitempty"`

	Runtime struct {
		ForceInterpreter        bool `json:"force_interpreter,omitempty"`            // If set, will use interpreter mode even on platforms with compiler support
		DoNotCloseOnContextDone bool `json:"do_not_close_on_context_done,omitempty"` // If unset, will close the module when the context is done and prevent any further calls to the module
		// Setting CompilationCache is not supported yet through JSON
	} `json:"runtime,omitempty"`
}
