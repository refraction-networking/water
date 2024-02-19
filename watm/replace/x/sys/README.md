# Replacement module for `golang.org/x/sys`

This module is a replacement for the `golang.org/x/sys` module. 

## What's wrong with the original module?

The original module starting from Go 1.21 relies on an addition to the `runtime` library that is not yet available in TinyGo (which has an overriding `runtime` library based on Go 1.20).

## What did we do to this module?

We slightly modified the part to make it not relying on the new `runtime` library when compiling towards WASI.