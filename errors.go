package water

import "fmt"

type WASMErrCode = int32

// WASMErrCode
const (
	NO_ERROR WASMErrCode = -iota
	GENERAL_ERROR
	INVALID_ARGUMENT
	INVALID_CONFIG
	INVALID_FD
	INVALID_FUNCTION
	DOUBLE_INIT
	FAILED_IO
	NOT_INITIALIZED
)

var (
	ErrGeneralError    = fmt.Errorf("general error")
	ErrInvalidArgument = fmt.Errorf("invalid argument")
	ErrInvalidConfig   = fmt.Errorf("invalid config")
	ErrInvalidFD       = fmt.Errorf("invalid file descriptor")
	ErrInvalidFunction = fmt.Errorf("invalid function")
	ErrDoubleInit      = fmt.Errorf("double init")
	ErrFailedIO        = fmt.Errorf("i/o operation failed")
	ErrNotInitialized  = fmt.Errorf("not initialized")
)

var mapWASMErrCode = map[WASMErrCode]error{
	NO_ERROR:         nil,
	GENERAL_ERROR:    ErrGeneralError,
	INVALID_ARGUMENT: ErrInvalidArgument,
	INVALID_CONFIG:   ErrInvalidConfig,
	INVALID_FD:       ErrInvalidFD,
	INVALID_FUNCTION: ErrInvalidFunction,
	DOUBLE_INIT:      ErrDoubleInit,
	FAILED_IO:        ErrFailedIO,
	NOT_INITIALIZED:  ErrNotInitialized,
}

func WASMErr(code WASMErrCode) error {
	if err, ok := mapWASMErrCode[code]; ok {
		return err
	}
	return fmt.Errorf("unrecognized error (%d)", code)
}
