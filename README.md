# W.A.T.E.R.: WebAssembly Transport Executable Reactor
[![Build Status](https://github.com/gaukas/water/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gaukas/water/actions/workflows/go.yml) 

W.A.T.E.R. provides a runtime environment for WebAssembly modules to run in and work as a application-layer transport protocol. It is designed to be highly portable and lightweight, and can be used as a replacement for pluggable transports.

## API 

Currently, W.A.T.E.R. provides a set of APIs relying on **WASI Preview 1 (wasip1)** snapshot. 

### Config
A `Config` is a struct that contains the configuration for a WASI instance. It is used to configure the WASI reactor before starting it. 

### RuntimeConn
A `RuntimeConn` is a `Conn` that represents a connection from the local user to a remote peer. Each living `RuntimeConn` encapsulates a running WASI instance. 
It process the data sent from the local user and send it to the remote peer, and vice versa.

A `RuntimeConn` interfaces `io.ReadWriteCloser` and is always and only spawned by a `RuntimeConnDialer`.

#### RuntimeConnDialer
A `RuntimeConnDialer` is a `Dialer` loaded with a `Config` that can dial for `RuntimeConn` as abstracted connections. Currently, it is just a wrapper around a `Config`. **It does not contain any running WASI instance.**

### RuntimeDialer _(TODO)_
A `RuntimeDialer` is a `Dialer` that dials for `RuntimeDialerConn`. Each living `RuntimeDialer` encapsulates a running WASI instance. It manages multiple `RuntimeDialerConn` instances created upon caller's request.

\* Not to be confused with [`RuntimeConnDialer`](#runtimeconndialer), a static dialer which creates `RuntimeConn` instances from `Config`.

#### RuntimeDialerConn
A `RuntimeDialerConn` is a sub-`Conn` spawned by a `RuntimeDialer` upon caller's request. It is a `Conn` that is dialed by a `RuntimeDialer` and is used to communicate with a remote peer. Multiple `RuntimeDialerConn` instances can be created from a single `RuntimeDialer`, which means they could be related to one single WASI instance.

\* Not to be confused with [`RuntimeConn`](#runtimeconn), an `io.ReadWriteCloser` that encapsulates a running WASI instance each.

## TODOs

- W.A.T.E.R. API
    - [x] `Config`
    - [x] `RuntimeConn`
        - [x] `RuntimeConnDialer`
    - [ ] `RuntimeDialer`
        - [ ] `RuntimeDialerConn`
- [x] Minimal W.A.T.E.R. WASI example
    - No background worker threads
- [ ] Multi-threaded W.A.T.E.R. WASI example
    - [ ] Background worker threads working 
