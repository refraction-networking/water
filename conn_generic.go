package water

import (
	"fmt"

	"github.com/gaukas/water/interfaces"
)

type coreDial = func(core interfaces.Core, network, address string) (interfaces.Conn, error)
type coreAccept = func(core interfaces.Core) (interfaces.Conn, error)

var (
	mapCoreDial   = make(map[string]coreDial)
	mapCoreAccept = make(map[string]coreAccept)
)

func RegisterDial(version string, dial coreDial) error {
	if _, ok := mapCoreDial[version]; ok {
		return fmt.Errorf("water: core dial context already registered for version %s", version)
	}
	mapCoreDial[version] = dial
	return nil
}

func RegisterAccept(version string, accept coreAccept) error {
	if _, ok := mapCoreAccept[version]; ok {
		return fmt.Errorf("water: core accept already registered for version %s", version)
	}
	mapCoreAccept[version] = accept
	return nil
}

func DialVersion(core interfaces.Core, network, address string) (interfaces.Conn, error) {
	for _, export := range core.Module().Exports() {
		if f, ok := mapCoreDial[export.Name()]; ok {
			return f(core, network, address)
		}
	}
	return nil, fmt.Errorf("water: core loaded a WASM module that does not implement any known version")
}

func AcceptVersion(core interfaces.Core) (interfaces.Conn, error) {
	for _, export := range core.Module().Exports() {
		if f, ok := mapCoreAccept[export.Name()]; ok {
			return f(core)
		}
	}
	return nil, fmt.Errorf("water: core loaded a WASM module that does not implement any known version")
}
