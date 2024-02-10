# `transport` submodules

This directory contains multiple submodule/packages. Each of which independently implements the driver for a WebAssembly Transport Module (WATM) spec version. 

Instead of putting all the code into the root of this module, we decided to split each 
mutually exclusive implementation of WATM driver into its own package. This way, users 
can pick and choose which implementation they want to import, and WATER can support 
multiple WATM driver implementations at the same time without forcing all users to 
include support for all versions.

## Import Rules

To prevent circular dependencies, the following import rules are enforced:

- Any module/package under this directory can import any other module/package in this **repository** (`water`).
- No module/package in this **repository** (`water`) may import any module/package from this directory.
    - excl. `*examples/*` and `*_test` modules/packages, since they are always terminals in the dependency graph.
    - excl. `transport/everyone` which is a special case as a shortcut to import every single transport module all at once. (planned feature)
