package water

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"

	"github.com/gaukas/water/configbuilder"
	"github.com/gaukas/water/internal/log"
	"google.golang.org/protobuf/proto"
)

// Config defines the configuration for the WATER Dialer/Config interface.
type Config struct {
	// TransportModuleBin contains the binary format of the WebAssembly
	// Transport Module.
	// In practice, this mandatory field could be populated by loading
	// a .wasm file, downloading from a remote host, or generating from
	// a .wat (WebAssembly Text Format) file.
	TransportModuleBin []byte

	// TransportModuleConfig optionally provides a configuration file to be pushed into
	// the WASM Transport Module.
	TransportModuleConfig TransportModuleConfig

	// NetworkDialerFunc specifies a func that dials the specified address on the
	// named network. This optional field can be set to override the Go
	// default dialer func:
	// 	net.Dial(network, address)
	NetworkDialerFunc func(network, address string) (net.Conn, error)

	// NetworkListener specifies a net.listener implementation that listens
	// on the specified address on the named network. This optional field
	// will be used to provide (incoming) network connections from a
	// presumably remote source to the WASM instance.
	//
	// Calling (*Config).Listen will override this field.
	NetworkListener net.Listener

	// ModuleConfigFactory is used to configure the system resource of
	// each WASM instance created. This field is for advanced use cases
	// and/or debugging purposes only.
	//
	// Caller is supposed to call c.ModuleConfig() to get the pointer to the
	// ModuleConfigFactory. If the pointer is nil, a new ModuleConfigFactory will
	// be created and returned.
	ModuleConfigFactory *WazeroModuleConfigFactory

	OverrideLogger *log.Logger // essentially a *slog.Logger, currently using an alias to flatten the version discrepancy
}

// Clone creates a deep copy of the Config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	wasmClone := make([]byte, len(c.TransportModuleBin))
	copy(wasmClone, c.TransportModuleBin)

	return &Config{
		TransportModuleBin:    wasmClone,
		NetworkDialerFunc:     c.NetworkDialerFunc,
		NetworkListener:       c.NetworkListener,
		TransportModuleConfig: c.TransportModuleConfig,
		ModuleConfigFactory:   c.ModuleConfigFactory.Clone(),
	}
}

// NetworkDialerFuncOrDefault returns the DialerFunc if it is not nil, otherwise
// returns the default net.Dial function.
func (c *Config) NetworkDialerFuncOrDefault() func(network, address string) (net.Conn, error) {
	if c.NetworkDialerFunc == nil {
		return net.Dial
	}

	return c.NetworkDialerFunc
}

// NetworkListenerOrDefault returns the NetworkListener if it is not nil,
// otherwise it panics.
func (c *Config) NetworkListenerOrPanic() net.Listener {
	if c.NetworkListener == nil {
		panic("water: network listener is not provided in config")
	}

	return c.NetworkListener
}

// WATMBinOrDefault returns the WATMBin if it is not nil, otherwise it panics.
func (c *Config) WATMBinOrPanic() []byte {
	if len(c.TransportModuleBin) == 0 {
		panic("water: WebAssembly Transport Module binary is not provided in config")
	}

	return c.TransportModuleBin
}

// ModuleConfig returns the ModuleConfigFactory. If the pointer is
// nil, a new ModuleConfigFactory will be created and returned.
func (c *Config) ModuleConfig() *WazeroModuleConfigFactory {
	if c.ModuleConfigFactory == nil {
		c.ModuleConfigFactory = NewWazeroModuleConfigFactory()

		// by default, stdout and stderr are inherited
		c.ModuleConfigFactory.InheritStdout()
		c.ModuleConfigFactory.InheritStderr()
	}

	return c.ModuleConfigFactory
}

// Listen creates a new Listener from the config on the specified network and
// address.
//
// For now, only TCP is supported.
//
// Deprecated: use ListenContext instead.
func (c *Config) Listen(network, address string) (Listener, error) {
	return c.ListenContext(context.Background(), network, address)
}

// ListenContext creates a new Listener from the config on the specified network
// and address with the given context.
//
// For now, only TCP is supported.
func (c *Config) ListenContext(ctx context.Context, network, address string) (Listener, error) {
	lis, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	config := c.Clone()
	config.NetworkListener = lis

	return NewListenerWithContext(ctx, config)
}

func (c *Config) Logger() *log.Logger {
	if c.OverrideLogger != nil {
		return c.OverrideLogger
	}

	return log.GetDefaultLogger()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Config) UnmarshalJSON(data []byte) error {
	var confJson configbuilder.ConfigJSON

	err := json.Unmarshal(data, &confJson)
	if err != nil {
		return err
	}

	tmBin, err := os.ReadFile(confJson.TransportModule.BinPath)
	if err != nil {
		return err
	}
	c.TransportModuleBin = tmBin

	if len(confJson.TransportModule.ConfigPath) > 0 {
		c.TransportModuleConfig, err = TransportModuleConfigFromFile(confJson.TransportModule.ConfigPath)
		if err != nil {
			return err
		}
	}

	if len(confJson.Network.Listener.Network) > 0 && len(confJson.Network.Listener.Address) > 0 {
		c.NetworkListener, err = net.Listen(confJson.Network.Listener.Network, confJson.Network.Listener.Address)
		if err != nil {
			return err
		}
	}

	c.ModuleConfigFactory = NewWazeroModuleConfigFactory()
	if len(confJson.Module.Argv) > 0 {
		c.ModuleConfigFactory.SetArgv(confJson.Module.Argv)
	}

	var envKeys []string
	var envValues []string
	for k, v := range confJson.Module.Env {
		envKeys = append(envKeys, k)
		envValues = append(envValues, v)
	}
	if len(envKeys) > 0 {
		c.ModuleConfigFactory.SetEnv(envKeys, envValues)
	}

	if confJson.Module.InheritStdin {
		c.ModuleConfigFactory.InheritStdin()
	}

	if confJson.Module.InheritStdout {
		c.ModuleConfigFactory.InheritStdout()
	}

	if confJson.Module.InheritStderr {
		c.ModuleConfigFactory.InheritStderr()
	}

	for k, v := range confJson.Module.PreopenedDirs {
		c.ModuleConfigFactory.SetPreopenDir(k, v)
	}

	return nil
}

// UnmarshalProto provides a way to unmarshal a protobuf message into a Config.
//
// The message definition is defined in configbuilder/pb/config.proto.
func (c *Config) UnmarshalProto(b []byte) error {
	var confProto configbuilder.ConfigProtoBuf

	unmarshalOptions := proto.UnmarshalOptions{
		AllowPartial: true,
	}
	err := unmarshalOptions.Unmarshal(b, &confProto)
	if err != nil {
		return err
	}

	// Parse TransportModuleBin
	c.TransportModuleBin = confProto.GetTransportModule().GetBin()
	if len(c.TransportModuleBin) == 0 {
		return errors.New("water: transport module binary is not provided in config")
	}

	// Parse TransportModuleConfig
	c.TransportModuleConfig = TransportModuleConfigFromBytes(confProto.GetTransportModule().GetConfig())

	// Parse NetworkListener
	listenerNetwork, listenerAddress := confProto.GetNetwork().GetListener().GetNetwork(), confProto.GetNetwork().GetListener().GetAddress()
	if len(listenerNetwork) > 0 && len(listenerAddress) > 0 {
		c.NetworkListener, err = net.Listen(listenerNetwork, listenerAddress)
		if err != nil {
			return err
		}
	}

	// Parse ModuleConfigFactory
	c.ModuleConfigFactory = NewWazeroModuleConfigFactory()
	if len(confProto.GetModule().GetArgv()) > 0 {
		c.ModuleConfigFactory.SetArgv(confProto.Module.Argv)
	}

	var envKeys []string
	var envValues []string
	for k, v := range confProto.GetModule().GetEnv() {
		envKeys = append(envKeys, k)
		envValues = append(envValues, v)
	}
	if len(envKeys) > 0 {
		c.ModuleConfigFactory.SetEnv(envKeys, envValues)
	}

	if confProto.GetModule().GetInheritStdin() {
		c.ModuleConfigFactory.InheritStdin()
	}

	if confProto.GetModule().GetInheritStdout() {
		c.ModuleConfigFactory.InheritStdout()
	}

	if confProto.GetModule().GetInheritStderr() {
		c.ModuleConfigFactory.InheritStderr()
	}

	for k, v := range confProto.GetModule().GetPreopenedDirs() {
		c.ModuleConfigFactory.SetPreopenDir(k, v)
	}

	return nil
}
