module github.com/refraction-networking/water

go 1.22.0

retract (
	v0.6.1 // bad submodule
	v0.6.0 // bad LICENSE naming
)

replace github.com/tetratelabs/wazero v1.7.0 => github.com/refraction-networking/wazero v1.7.0-w

require (
	github.com/karelbilek/wazero-fs-tools v0.0.0-20240317201741-fc5622f5bd12
	github.com/tetratelabs/wazero v1.7.0
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	google.golang.org/protobuf v1.33.0
)

require github.com/blang/vfs v1.0.0 // indirect
