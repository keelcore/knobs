# Roadmap

## Released

- **v0.8.0:** the Go package: the layers from code to runtime, schema validation, secrets, connection overrides,
  module attachment, the conformance suite, the schema bump tool, and the signed release.

## Next

- SLSA build provenance for the module zip on every release.
- Module owners and versions in the master schema; the committed-schema and single-major checks (`knobstest`).

## Planned

- Implementations in Rust, TypeScript, Python and C/C++, each against the same `spec/` and published to its
  language's registry (crates.io, npm, PyPI, signed C/C++ archives), with one version for every package.
