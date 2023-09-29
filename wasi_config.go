package water

import "github.com/bytecodealliance/wasmtime-go/v13"

// WASIConfigFactory creates wasmtime.WasiConfig.
// Since WasiConfig cannot be cloned, we will instead save
// all the repeated setup functions in a slice and call them
// on newly created wasmtime.WasiConfig when needed.
type WASIConfigFactory struct {
	setupFuncs []func(*wasmtime.WasiConfig) error // if any of these functions returns an error, the whole setup will fail.
}

func NewWasiConfigFactory() *WASIConfigFactory {
	return &WASIConfigFactory{
		setupFuncs: make([]func(*wasmtime.WasiConfig) error, 0),
	}
}

// GetConfig sets up and returns the finished wasmtime.WasiConfig.
//
// If the setup fails, it will return nil and an error.
func (wcf *WASIConfigFactory) GetConfig() (*wasmtime.WasiConfig, error) {
	wasiConfig := wasmtime.NewWasiConfig()
	if wcf != nil && wcf.setupFuncs != nil {
		for _, f := range wcf.setupFuncs {
			if err := f(wasiConfig); err != nil {
				return nil, err
			}
		}
	}
	return wasiConfig, nil
}

func (wcf *WASIConfigFactory) SetArgv(argv []string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetArgv(argv)
		return nil
	})
}

func (wcf *WASIConfigFactory) InheritArgv() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritArgv()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetEnv(keys, values []string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetEnv(keys, values)
		return nil
	})
}

func (wcf *WASIConfigFactory) InheritEnv() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritEnv()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStdinFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdinFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStdin() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdin()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStdoutFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdoutFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStdout() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdout()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStderrFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStderrFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStderr() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStderr()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetPreopenDir(path string, guestPath string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.PreopenDir(path, guestPath)
		return nil
	})
}
