//go:build nogcfix

package water

// If the program is compiled with `go build -tags nogcfix`, the
// GC fix mentioned in gcfix.go will not be applied. Unexpected
// GC behavior is expected ;)
const GCFIX bool = false
