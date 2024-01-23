module github.com/gaukas/water

go 1.20

replace github.com/tetratelabs/wazero v1.6.0 => github.com/gaukas/wazero v1.6.4-w

require (
	github.com/tetratelabs/wazero v1.6.0
	golang.org/x/exp v0.0.0-20231226003508-02704c960a9b
	google.golang.org/protobuf v1.32.0
)
