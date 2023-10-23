# W.A.T.E.R.: WebAssembly Transport Executables Runtime
[![License](https://img.shields.io/badge/License-Apache_2.0-yellowgreen.svg)](https://opensource.org/licenses/Apache-2.0) 
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fgaukas%2Fwater.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fgaukas%2Fwater?ref=badge_shield&issueType=license)
[![Build Status](https://github.com/gaukas/water/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gaukas/water/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gaukas/water.svg)](https://pkg.go.dev/github.com/gaukas/water)
[![DeepSource](https://app.deepsource.com/gh/gaukas/water.svg/?label=resolved+issues&show_trend=true&token=SonUOOtyjJHnPuIdEBGZp4zx)](https://app.deepsource.com/gh/gaukas/water/)

W.A.T.E.R. provides a runtime environment to run WebAssembly Transport Modules(WATM) as a pluggable application-layer transport protocol provider. It is designed to be highly portable and lightweight, which serves as an alternative approach for pluggable transports.

# Usage

## API 
Based on **WASI Preview 1 (wasip1)** snapshot, currently W.A.T.E.R. provides a set of provide a `net`-like APIs, including `Dialer`, `Listener` and `Relay`.

### Dialer

A `Dialer` could be used to dial a remote address upon `Dial()` and return a `net.Conn` back to the caller once the connection is established. Caller could use the `net.Conn` to read and write data to the remote address and the data will be processed by a WebAssembly instance.

`Dialer` is used to _upgrade_ (encode, wrap) the caller's connection into an outbound, transport-wrapped connection.

### Listener

A `Listener` could be used to listen on a local address. Upon `Accept()`, it returns a `net.Conn` back once an incoming connection is accepted from the wrapped listener. Caller could use the `net.Conn` to read and write data to the remote address and the data will be processed by a WebAssembly instance.

`Listener` is used to _downgrade_ (decode, unwrap) the incoming transport-wrapped connection into caller's raw connection.

### Relay

A `Relay` somewhat combines the role of `Dialer` and `Listener`. It could be used to listen on a local address and dial a remote address and automatically `Accept()` the incoming connections, feed them into the WebAssembly Transport Module and `Dial()` a pre-defined remote address. Without any caller interaction, the `Relay` will automatically handle the data transmission between the two ends.

`Relay` is used to _upgrade_ the incoming connection into an outbound, transport-wrapped connection.

## Example

See [examples](./examples) for example usecase of W.A.T.E.R. API, including `Dialer`, `Listener` and `Relay`.