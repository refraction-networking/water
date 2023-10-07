# W.A.T.E.R.: WebAssembly Transport Executable Reactor
[![License](https://img.shields.io/badge/License-Apache_2.0-yellowgreen.svg)](https://opensource.org/licenses/Apache-2.0) [![Build Status](https://github.com/gaukas/water/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gaukas/water/actions/workflows/go.yml) 

W.A.T.E.R. provides a runtime environment for WebAssembly modules to run in and work as a application-layer transport protocol. It is designed to be highly portable and lightweight, and can be used as a replacement for pluggable transports.

## API 

Currently, W.A.T.E.R. provides a set of APIs based on **WASI Preview 1 (wasip1)** snapshot. 

### Config
A `Config` is a struct that contains the configuration for a WASI instance. It is used to configure the WASI reactor before starting it. 

### Dialer 

A `Dialer` could be used to dial a remote address upon `Dial()` and return a `net.Conn` back to the caller once the connection is established. Caller could use the `net.Conn` to read and write data to the remote address and the data will be processed by a WebAssembly instance.

### Listener

A `Listener` could be used to listen on a local address. Upon `Accept()`, it returns a `net.Conn` back once an incoming connection is accepted from the wrapped listener. Caller could use the `net.Conn` to read and write data to the remote address and the data will be processed by a WebAssembly instance.

### Server

A `Server` somewhat combines the role of `Dialer` and `Listener`. It could be used to listen on a local address and dial a remote address and automatically `Accept()` the incoming connections, feed them into the WebAssembly instance and `Dial()` the pre-defined remote address. Without any caller interaction, the `Server` will automatically* handle the data transmission between the two ends.

***TODO: Server could not be realistic until WASI multi-threading or blocking mainloop is supported**