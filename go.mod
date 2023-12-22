module github.com/gaukas/water

go 1.20

replace github.com/bytecodealliance/wasmtime-go/v13 v13.0.0 => github.com/refraction-networking/wasmtime-go/v13 v13.0.0

replace github.com/tetratelabs/wazero v1.5.0 => github.com/gaukas/wazero v1.5.0-w

require (
	github.com/bytecodealliance/wasmtime-go/v13 v13.0.0
	github.com/tetratelabs/wazero v1.5.0
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9
)
