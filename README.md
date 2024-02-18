# W.A.T.E.R.: WebAssembly Transport Executables Runtime
[![License](https://img.shields.io/github/license/gaukas/water)](https://github.com/refraction-networking/water/blob/master/LICENSE)
[![FOSSA](https://app.fossa.com/api/projects/git%2Bgithub.com%2Frefraction-networking%2Fwater.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Frefraction-networking%2Fwater?ref=badge_shield&issueType=license)
[![CI](https://github.com/refraction-networking/water/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/refraction-networking/water/actions/workflows/go.yml)
[![Go Doc](https://pkg.go.dev/badge/github.com/refraction-networking/water.svg)](https://pkg.go.dev/github.com/refraction-networking/water)

<div style="width: 100%; height = 160px">
    <div style="width: 75%; height: 150px; float: left;"> 
        WATER-go provides a Go runtime for WebAssembly Transport Modules(WATM) as a pluggable
        application-layer transport protocol provider. It is designed to be highly portable and
        lightweight, allowing for rapidly deployable pluggable transports. While other pluggable
        transport implementations require a fresh client deployment (and app-store review) to update
        their protocol WATER allows <b><u>dynamic delivery of new transports</u></b> in real time
        over the network or out-of-band.<br />
        <br />
    </div>
    <div style="margin-left: 80%; height: 150px;"> 
        <img src=".github/assets/logo_v0.svg" alt="WATER wasm transport" align="right">
    </div>
</div>

To build a WATM in Go, please refer to [watm](https://github.com/refraction-networking/water/watm) for examples and helper libraries interfacing Pluggable Transports-like interfaces. Official Go compiler is currently not supported until further notice.

You can contact one of developers personally via gaukas.wang@colorado.edu, or simply [opening an issue](https://github.com/refraction-networking/water/issues/new). 

The Rust implementation of the runtime library and information about writing, building, and using WebAssembly Transport Modules(WATM) from Rust can be found in [water-rs](https://github.com/refraction-networking/water-rs). 

### Cite our work

If you quoted or used our work in your own project/paper/research, please cite our paper [Just add WATER: WebAssembly-based Circumvention Transports](https://www.petsymposium.org/foci/2024/foci-2024-0003.pdf), which is published in the proceedings of Free and Open Communications on the Internet (FOCI) in 2024 issue 1, pages 22-28.

<details>
  <summary>BibTeX</summary>
    
  ```bibtex
    @inproceedings{water-foci24,
        author = {Chi, Erik and Wang, Gaukas and Halderman, J. Alex and Wustrow, Eric and Wampler, Jack},
        year = {2024},
        month = {02},
        number = {1},
        pages = {22-28},
        title = {Just add {WATER}: {WebAssembly}-based Circumvention Transports},
        howpublished = "\url{https://www.petsymposium.org/foci/2024/foci-2024-0003.php}",
        publisher = {PoPETs},
        address = {Virtual Event},
        series = {FOCI '24},
        booktitle = {Free and Open Communications on the Internet},
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

## Versioning

W.A.T.E.R. is designed to be future-proof with the automated multi-version WebAssembly Transport Module(WATM) support. In order to minimize the size of compiled application binaries importing `water`, the support for each WATM version is implemented in separate sub-packages and by default none will be enabled. The developer MUST manually enable each version to be supported by importing the corresponding package: 

```go
import (
	// ...

	_ "github.com/refraction-networking/water/transport/v0"

	// ...
)
```

Otherwise, it is possible that the W.A.T.E.R. runtime cannot determine the version of the WATM and therefore fail to select the corresponding runtime: 

```go
panic: failed to listen: water: listener version not found
```

### Customizable Version

_TODO: add documentations for customizable WATM version._

## Components

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

	dialer, _ := water.NewDialerWithContext(context.Background(), config)
	conn, _ := dialer.DialContext(context.Background(),"tcp", remoteAddr)
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

	lis, _ := config.ListenContext(context.Background(), "tcp", localAddr)
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

	relay, _ := water.NewRelayWithContext(context.Background(), config)

	relay.ListenAndRelayTo("tcp", localAddr, "tcp", remoteAddr) // blocking
```

## Example

See [examples](./examples) for example usecase of W.A.T.E.R. API, including `Dialer`, `Listener` and `Relay`.

# Cross-platform Support

W.A.T.E.R. is designed to be cross-platform (and cross-architecture). 
Currently, it supports the following platforms: 

|       Target       | Compiles? | Tests Pass? |
| ------------------ | --------- | ----------- | 
| linux/amd64        | ✅        | ✅         |
| linux/arm64        | ✅        | ✅         |
| linux/riscv64      | ✅        | ✅         |
| macos/amd64        | ✅        | ✅         |
| macos/arm64        | ✅        | ✅         |
| windows/amd64      | ✅        | ✅         |
| windows/arm64      | ✅        | ❓         |
| others             | ❓        | ❓         |

## Acknowledgments

* We thank [GitHub.com](https://github.com) for providing GitHub Actions runners for all targets below:
	* `linux/amd64` on Ubuntu Latest
	* `linux/arm64` via [docker/setup-qemu-action](https://github.com/docker/setup-qemu-action)
	* `linux/riscv64` via [docker/setup-qemu-action](https://github.com/docker/setup-qemu-action)
	* `macos/amd64` on macOS 12
	* `macos/arm64` on macOS 14
	* `windows/amd64` on Windows Latest

* We thank [FlyCI.net](https://www.flyci.net) for providing GitHub Actions runners on `macos/arm64` (Apple M1) _in the past_. (We switched to GitHub's `macos-14` runner as of Jan 31 2024)

We are currently actively looking for a CI provider for more target platforms. Please reach out and let us know if you would like to help.
