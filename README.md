# W.A.T.E.R.: WebAssembly Transport Executables Runtime
![License](https://img.shields.io/github/license/gaukas/water?label=License)
[![FOSSA](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fgaukas%2Fwater.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fgaukas%2Fwater?ref=badge_shield&issueType=license)
[![CI](https://github.com/gaukas/water/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gaukas/water/actions/workflows/go.yml)
[![Go Doc](https://pkg.go.dev/badge/github.com/gaukas/water.svg)](https://pkg.go.dev/github.com/gaukas/water)

<div style="width: 100%; height = 160px">
    <div style="width: 75%; height: 150px; float: left;"> 
        WATER-go provides a Go runtime for WebAssembly Transport Modules(WATM) as a pluggable
        application-layer transport protocol provider. It is designed to be highly portable and
        lightweight, allowing for rapidly deployable pluggable transports. While other pluggable
        transport implementations require a fresh client deployment (and app-store review) to update
        their protocol WATER allows <b><u>dynamic delivery of new transports</u></b> in real time
        over the network.<br />
        <br />
    </div>
    <div style="margin-left: 80%; height: 150px;"> 
        <img src=".github/assets/logo_v0.svg" alt="WATER wasm transport" align="right">
    </div>
</div>

The Rust implementation of the runtime library and information about writing, buiding, and sharing WebAssembly Transport Modules(WATM) can be found in [water-rs](https://github.com/erikziyunchi/water-rs). 

### Citation Information

If you quoted or used our work, please cite our paper [Just add WATER: WebAssembly-based Circumvention Transports](https://arxiv.org/pdf/2312.00163.pdf) in your work.

<details>
  <summary>BibTeX</summary>
    
  ```bibtex
  @misc{chi2023just,
    title={Just add WATER: WebAssembly-based Circumvention Transports}, 
    author={Erik Chi and Gaukas Wang and J. Alex Halderman and Eric Wustrow and Jack Wampler},
    year={2023},
    eprint={2312.00163},
    archivePrefix={arXiv},
    primaryClass={cs.CR}
  }
  ```
</details>

## Be Water

> Empty your mind, be formless, shapeless, like water. If you put water into a cup, it becomes the cup. You put water into a bottle and it becomes the bottle. You put it in a teapot, it becomes the teapot. Now, water can flow or it can crash. Be water, my friend.
>
> -- Bruce Lee

## Contents

This repo contains a Go package `water`, which implements the runtime library used to interact with `.wasm` WebAssembly Transport Modules(WATM). 

# Usage

<!-- ## API  -->
Based on **WASI Snapshot Preview 1** (_wasip1_), currently W.A.T.E.R. provides a set of `net`-like APIs via `Dialer`, `Listener` and `Relay`.

### Dialer

A `Dialer` connects to a remote address and returns a `net.Conn` to the caller if the connection can
be successfully established. The `net.Conn` then provides tunnelled read and write to the remote
endpoint with the WebAssembly module wrapping / encrypting / transforming the traffic.

`Dialer` is used to encapsulate the caller's connection into an outbound, transport-wrapped
connection.

```go
	wasm, _ := os.ReadFile("./examples/v0/plain/plain.wasm")

	config := &water.Config{
		TransportModuleBin: wasm,
	}

	dialer, _ := water.NewDialer(config)
	conn, _ := dialer.Dial("tcp", remoteAddr)
	// ...
```

### Listener

A `Listener` listens on a local address for incoming connections which  it `Accept()`s, returning
a `net.Conn` for the caller to handle. The WebAssembly Module is responsible for the initial
accpt allowing it to implement both the server side handshake as well as any unwrap / decrypt
/reform operations required to validate and re-assemble the stream. The `net.Conn` returned provides
the normalized stream, and allows the caller to write back to the client with the WebAssembly module
encoding / encrypting / transforming traffic to be obfuscated on the wire on the way to the remote 
client.


`Listener` is used to decapsulate incoming transport-wrapped connections for the caller to handle,
managing the tunnel obfuscation once a connection is established.

```go
	wasm, _ := os.ReadFile("./examples/v0/plain/plain.wasm")

	config := &water.Config{
		TransportModuleBin: wasm,
	}

	lis, _ := config.Listen("tcp", localAddr)
	defer lis.Close()
	log.Printf("Listening on %s", lis.Addr().String())

	for {
		conn, err := lis.Accept()
		handleConn(conn)
	}

	// ...
```

### Relay

A `Relay` combines the role of `Dialer` and `Listener`. It listens on a local address `Accept()`-ing
incoming connections and `Dial()`-ing the remote endpoints on establishment. The connecions are
tunneled through the WebAssembly Transport Module allowing the module to handshake, encode,
transform, pad, etc. without any caller interaction. The internal `Relay` manages  the incoming
connections as well as the associated outgoing connectons.

`Relay` is a managed many-to-many handler for incoming connections that uses the WebAssemble module
to tunnel traffic.

```go
	wasm, _ := os.ReadFile("./examples/v0/plain/plain.wasm")

	config := &water.Config{
		TransportModuleBin: wasm,
	}

	relay, _ := water.NewRelay(config)

	relay.ListenAndRelayTo("tcp", localAddr, "tcp", remoteAddr) // blocking
```

## Versioning

W.A.T.E.R. is designed to support multiple versions of the WebAssembly Transport Module(WATM) specification at once. The current maximum supported version is `v0`. 

To minimize the size of compiled application binaries importing the `water` package, the support for each version is implemented in separate sub-packages. Developers should import the sub-package that matches the version of the WATM they expect to use.

```go
import (
	// ...

	_ "github.com/gaukas/water/transport/v0"

	// ...
)
```

Otherwise, it is possible that the W.A.T.E.R. runtime cannot determine the version of the WATM and therefore fail to select the corresponding runtime: 

```go
panic: failed to listen: water: listener version not found
```

## Example

See [examples](./examples) for example usecase of W.A.T.E.R. API, including `Dialer`, `Listener` and `Relay`.

# Cross-platform Support

W.A.T.E.R. is designed to be cross-platform (and cross-architecture). 
Currently, it supports the following platforms: 

| Platform | Architecture | Compiles?† | Tests Pass? |
| -------- | ------------ | ---------- | ----------- | 
| Linux    | amd64        | ✅         | ✅         |
| Linux    | aarch64<sup>†</sup>     | ❓         | ❓         |
| Linux    | riscv64<sup>†</sup>     | ❓         | ❓         |
| macOS    | amd64        | ✅         | ✅         |
| macOS    | aarch64<sup>†</sup>     | ❓         | ❓         |
| Windows  | amd64<sup>‡</sup>       | ✅         | ❌         |
| Windows  | aarch64<sup>†‡</sup>    | ❓         | ❌         |

<sup>†</sup> CI covering compilation and testing on non-amd64 platforms are planned and currently being worked on. Community help would be greatly appreciated. <br>
<sup>‡</sup> Windows support is currently unavailable due to the lack of async I/O support from Go (via `wazero`). 