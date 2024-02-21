# `watm`: WebAssembly Transport Module
![Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-green)
![GPLv3](https://img.shields.io/badge/License-GPL--3.0-red)
[![Test](https://github.com/refraction-networking/water/actions/workflows/watm.yml/badge.svg?branch=master)](https://github.com/refraction-networking/water/actions/workflows/watm.yml)
[![Release Status](https://github.com/refraction-networking/water/actions/workflows/release.yml/badge.svg)](https://github.com/refraction-networking/water/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/refraction-networking/water/watm.svg)](https://pkg.go.dev/github.com/refraction-networking/water/watm)

This repository contains tools for building WebAssembly Transport Modules (WATMs) for [water](https://github.com/gaukas/water) project. 

# License

This project is dual-licensed under both the Apache 2.0 license and the GPLv3 license. The license applies differently depending on how this project is used.

- **Apache 2.0**: applies for the project itself, and all of its submodules EXCEPT examples under `watm` module.
- **GPLv3** applies when your project uses the code from the examples provided by the `watm` module, including but not limited to when you modify and redistribute the example code, or even use it for a non-water scenario. However, if you decide to distribute the examples in a compiled form (i.e., the `.wasm` file), you are free to use the compiled output without a problem.

In short, if you import `water` and `watm` module and build your own WATM out of it, you are free to use the Apache 2.0 license. If you redistribute the code of examples in `watm` module, you are subject to the GPLv3 license.