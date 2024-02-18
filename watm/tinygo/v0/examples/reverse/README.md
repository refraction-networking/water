# Example: `reverse.wasm` 

This example shows how to build a minimal WATM with TinyGo which reverse the received string.

## Build

Go 1.20/1.21 is required to build this example until TinyGo supports Go 1.22.

### Debug

```bash
tinygo build -o reverse.wasm -target=wasi -scheduler=none -gc=conservative .
```

### Release

```bash
tinygo build -o reverse.wasm -target=wasi -no-debug -scheduler=none -gc=conservative .
```