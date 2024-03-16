module github.com/refraction-networking/water

go 1.20

retract (
	v0.6.1 // bad submodule
	v0.6.0 // bad LICENSE naming
)

replace github.com/tetratelabs/wazero v1.7.0 => github.com/refraction-networking/wazero v1.7.0-w

require (
	github.com/tetratelabs/wazero v1.7.0
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	google.golang.org/protobuf v1.33.0
)
