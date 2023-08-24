# W.A.T.E.R.: WebAssembly Transport Executable Reactor

W.A.T.E.R. provides a runtime environment for WebAssembly modules to run in and work as a application-layer transport protocol. It is designed to be highly portable and lightweight, and can be used as a replacement for pluggable transports.

## API 

Currently, W.A.T.E.R. provides a set of APIs relying on **WASI Preview 1 (wasip1)** snapshot. 

### Config

A `Config` is a struct that contains the configuration for a WASI instance. It is used to configure the WASI reactor before starting it. See [Config](https://github.com/