# `wasip1` package

This package is mostly copied from Go standard library `syscall` package for `wasip1` target, and is a duplicate to `watm/wasip1` package. We choose to maintain a separate package for `wasip1` target to avoid import cycle (`watm` may import `water` for some example/demo code).