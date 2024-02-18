# Example: `uTLS.wasm`

This example shows how to build a fully functional TLS client with TinyGo from [uTLS](https://github.com/refraction-networking/utls/tree/wasm). 

## Build

Go 1.20/1.21 is required to build this example until TinyGo supports Go 1.22.

```bash
tinygo build -o utls.wasm -target=wasi -scheduler=asyncify -gc=conservative -tags=purego .
```

### Debug

```bash
tinygo build -o utls.wasm -target=wasi -scheduler=asyncify -gc=conservative -tags=purego .
```

### Release

```bash
tinygo build -o utls.wasm -target=wasi -no-debug -scheduler=asyncify -gc=conservative -tags=purego .
```

## Dependencies

The `utls` imported must be from the `wasm` branch of [uTLS](https://github.com/refraction-networking/utls/tree/wasm). You may use a replace directive in `go.mod` to your local clone of uTLS, or use a tagged version with suffix `-wasm` (e.g., `v1.6.2-wasm`) to make sure it is tagged from the correct branch.