module github.com/gaukas/water

go 1.20

replace github.com/tetratelabs/wazero v1.6.0 => github.com/gaukas/wazero v1.6.5-w

require (
	github.com/tetratelabs/wazero v1.6.0
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	google.golang.org/protobuf v1.32.0
)
