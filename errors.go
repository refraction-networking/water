package water

import "fmt"

type WASMErrCode = int32

// WASMErrCode
const (
	NO_ERROR WASMErrCode = -iota
	NO_FILE_DESCRIPTOR_AVAILABLE
	CONFIG_REJECTED
)

var mapWASMErrCode = map[WASMErrCode]string{
	CONFIG_REJECTED: "WASM module rejected the configuration",
}

func WASMErr(code WASMErrCode) error {
	if code == NO_ERROR {
		return nil
	}

	return fmt.Errorf("%s", mapWASMErrCode[code])
}
