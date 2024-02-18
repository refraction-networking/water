# `watm`: WebAssembly Transport Module

[![Test](https://github.com/refraction-networking/water/actions/workflows/watm.yml/badge.svg?branch=master)](https://github.com/refraction-networking/water/actions/workflows/watm.yml)
[![Release Status](https://github.com/refraction-networking/water/actions/workflows/release.yml/badge.svg)](https://github.com/refraction-networking/water/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/refraction-networking/water/watm.svg)](https://pkg.go.dev/github.com/refraction-networking/water/watm)

This repository contains tools for building WebAssembly Transport Modules (WATMs) for [water](https://github.com/gaukas/water) project. 

## Dual License

This module is licensed under both the Apache 2.0 license and the GPLv3 license by inheritance from the `water` project. The license applies differently depending on how this module is used. 

- **Apache 2.0**: **ONLY** when this module is distributed as a part of the `water` project, and used to build a WebAssembly Transport Module for the `water` project.
- **GPLv3** applies otherwise, which means if you modify and redistribute this module or use it for a non-water scenario, you MUST also open source your modification under the GPLv3 license and forfeit the right to use the Apache 2.0 license.

The provided example WATMs are ALWAYS licensed under the GPLv3 license no matter the use case, which means you may use the compilation output AS-IS freely, or modify and redistribute them under the GPLv3 license.