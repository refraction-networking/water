## `configbuilder` package

This directory contains a `configbuilder` package used to assist in building `water.Config` from serialized data such as JSON or Protobuf.

## Import Rules

To prevent circular dependencies, the following import rules are enforced:

- Any module/package under this directory can ONLY import packages under this directory (e.g., `configbuilder/pb`) or from the `internal` directory (e.g. `internal/log`).
- Any module/package from this **repository** other than `internal` can import any module/package from this directory.
