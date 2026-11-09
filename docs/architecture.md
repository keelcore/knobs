# Architecture

## Shape

knobs is a library. One contract (`spec/`) is implemented per language; the Go implementation is the package at the
repository root.

| Part | Where | Role |
| --- | --- | --- |
| Contract | `spec/README.md`, `spec/schema/`, `spec/cases/` | names, schema keywords, layers, errors, conformance cases |
| Engine | `engine.go` | `New`, `Attach`, `Add`, `AddStandard`, `Resolve` |
| Schemas | `module.go`, `rules.go`, `strict.go`, `owner.go` | read module and application schemas, compile and enforce them with `santhosh-tekuri/jsonschema/v6`, fail any keyword a schema's draft does not define, build the master schema, owners and versions |
| Layers | `layer.go` | the `Layer` interface and the built-in layers: code, schema, files, env, cli, runtime |
| Values | `value.go` | text conversion, typed checks, rules, clamping |
| Configuration | `config.go`, `bind.go`, `secret.go` | the resolved `Config`, typed accessors, binding, `Secret` |
| Connections | `connection.go` | `WithConnection`: the per-connection overlay |
| Release rule | `schemabump.go`, `cmd/knobs-schema-bump` | the semver bump a schema change calls for |
| Test helpers | `knobstest/` | the committed-schema check and the single-major check |
| Harness | `internal/conform`, `cmd/knobs-conform` | arguments in, JSON out, for the conformance suite |

## Resolution

`Resolve` walks the layers in the order they were added. Each layer `Load`s a list of settings against the attached
schemas; the engine sorts them by key, checks each (unknown key, secret policy, type, rules) and writes it over the
values below. The first error stops resolution and names its layer, key and the key's owner. A module's own files are
layers inserted right after the application's file of the same layer. After the last layer, required keys are
checked. The result is an immutable `Config`; `Resolve` closes the engine, so later `Attach` and `Add` calls fail.

## Connections

`WithConnection` returns a `Config` that points at the process `Config` and holds a sorted slice of its own overrides.
Lookups check the overrides by binary search, then the process values. Nothing is copied, so a connection's memory is
proportional to its overrides, and any number of goroutines read the shared process `Config` without locks.

## Schema checks

A module schema may use only the keywords knobs enforces; the engine checks types, bounds, enums, lengths and the
`x-knobs-*` markings itself, with the same error codes in every language. Defaults are checked against their own
rules when the schema is attached.

## Owners

A module schema's `x-knobs-owner` names the code module that defines it. The master schema keeps it and adds
`x-knobs-owner-version`, read from the binary's build information (`runtime/debug.ReadBuildInfo`), and errors on the
module's keys name `owner@version`.
