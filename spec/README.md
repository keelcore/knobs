# knobs specification

This directory is the contract every implementation follows. A behavior is specified when a case in `cases/` shows
it; `tests/conformance.bats` runs every case against every implementation.

## Names

An application is named once (`app`); a schema names its module (`x-knobs-module`); each property is a knob. Names
are lowercase snake case (`^[a-z][a-z0-9_]*$`). The module name `knobs` is reserved for the engine's own settings.

| Form | Pattern | Example (app `demo`, module `http`, knob `read_timeout`) |
| --- | --- | --- |
| key | `<app>.<module>.<name>` | `demo.http.read_timeout` |
| file | `<app>:` → `<module>:` → `<name>:` | `demo:` → `http:` → `read_timeout:` |
| env | `<APP>_<MODULE>_<NAME>`, or the property's `x-knobs-env` | `DEMO_HTTP_READ_TIMEOUT` |
| cli | `--knob <app>.<module>.<name>=<value>` | `--knob demo.http.read_timeout=5s` |

## Schemas

A module schema is a JSON Schema (draft 2020-12) object:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "x-knobs-module": "http",
  "type": "object",
  "additionalProperties": false,
  "required": ["port"],
  "properties": {
    "port": { "type": "integer", "default": 8080, "minimum": 1, "maximum": 65535, "x-knobs-connection": true }
  }
}
```

Every keyword is enforced; a schema using any other keyword fails with `schema_invalid`, so no rule in a schema goes
unenforced.

| Where | Keywords |
| --- | --- |
| module | `$schema`, `$id`, `title`, `description`, `x-knobs-module`, `x-knobs-owner`, `type` (`object`), `additionalProperties` (`false`), `required`, `properties` |
| property | `type` (`string`, `integer`, `number`, `boolean`), `title`, `description`, `default`, `enum`, `minimum`, `maximum`, `minLength`, `maxLength`, `x-knobs-type` (`duration`), `x-knobs-secret`, `x-knobs-connection`, `x-knobs-env` |

- `minimum` and `maximum` apply to `integer` and `number`; `minLength` and `maxLength` to `string`.
- `x-knobs-type: duration` needs `type: string`; a value is a Go-style duration: one or more
  `<digits>[.<digits>]<unit>`, unit one of `ns`, `us`, `µs`, `ms`, `s`, `m`, `h`, optionally signed (`1m30s`).
- A `default` satisfies its own property's rules. A secret has no `default`, and is never `x-knobs-connection`.
- Two schemas with the same module fail with `module_conflict`.
- `x-knobs-owner` names the code module that defines the schema (a Go module path, a crate, a package). The master
  schema keeps it and adds `x-knobs-owner-version`, the version of that module the program links, when the language
  can tell; an error on one of the module's keys names the owner in its message.

## Layers

Lowest to highest precedence:

| Layer | Source | Secrets |
| --- | --- | --- |
| `code` | values the application sets in code | a key marked secret fails: `secret_in_code` (a credential in code is hardcoded) |
| `schema` | each property's `default`, when the schema gives one | never (a secret has no default) |
| `global` | the global file, then each module's global file | a key marked secret fails: `secret_in_plain_file` |
| `user` | the user file, then each module's user file | a key marked secret fails: `secret_in_plain_file` |
| `secrets` | the secrets file | every value from it is secret |
| `env` | the environment | allowed |
| `cli` | `--knob` arguments | a key marked secret fails: `secret_on_cli` (argv is visible to other users) |
| `runtime` | values the application sets while running | allowed |
| `connection` | the values the client sends with a connection, per connection | never: a secret is not connection-overridable |

- **Files** are YAML 1.2 mappings with the application name as their only root key, except a module's own file,
  which holds that module's mapping directly. Values keep their YAML types: `port: "8080"` is a string. A file missing
  at its default location is skipped; a file named explicitly that is missing fails with `file_not_found`.
  A module's own files are optional: missing, they are skipped. An `integer` property accepts a YAML number with no
  fractional part; a `number` property accepts any YAML number.
- **The secrets file** is its owner's alone, or it fails with `secrets_file_permissions`. Where file modes carry access
  rights (Linux, macOS), no group or other bit is set. On Windows, where they do not, its owner is the current user,
  SYSTEM or Administrators, and its DACL grants access to those three only.
- **env** reads only the variables named by the schemas; an empty value counts as unset.
- **code** values are typed, as files are: `9000` is an integer, `"9000"` a string. A schema `default` overrides a code
  value for the same key; a code value is the baseline for a key whose schema gives no default.
- **env, cli, runtime and connection** values are text, converted to the property's type: `integer` is optional `-` and
  digits; `number` is a decimal number; `boolean` is `true` or `false`; `string` is the text as given.
- **Every layer is validated** against the schemas before it is merged, so an error names its layer and key.
- **required** keys are set after the process layers (all but `connection`) merge, or resolution fails with
  `missing_required`.

## Connections

The layers from `code` to `runtime` are the process configuration, resolved once. A connection carries the
caller's state: a set of key-to-text values, and whether the connection is authorized for overrides. The caller
sends that state with every connection (as connection headers, call metadata or anything else); the application
maps it to values, and knobs never sees the transport. knobs keeps no state between connections: the same values and
authorization give the same result on any node. Each value is checked in this order:

1. Not authorized: `rejected_unauthorized`.
2. No schema defines the key, it is not `x-knobs-connection`, or it is secret: `rejected_not_overridable`.
3. The text does not convert to the property's type, is outside `enum`, or breaks `minLength`/`maxLength`:
   `rejected_malformed`.
4. A number outside `minimum`/`maximum` is set to the nearer bound: `clamped`.
5. Otherwise: `applied`.

## Errors

An error is reported as `{"error": {"code", "layer", "key"}}`; `key` is the dotted key when the error concerns one,
and is absent otherwise. Codes: `schema_invalid`, `module_conflict`, `file_not_found`, `file_parse`, `unknown_key`,
`invalid_type`, `out_of_range`, `not_in_enum`, `invalid_length`, `secret_in_plain_file`, `secret_on_cli`,
`secrets_file_permissions`, `secret_in_code`, `missing_required`, `invalid_argument`. Messages are free text, never
part of the contract, and never contain a secret value. When several errors occur, the first is reported: layers in
precedence order, keys in sorted order within a layer.

## The conformance harness

Each implementation builds `knobs-conform`:

```text
knobs-conform --app NAME --schema FILE... [--global FILE] [--user FILE] [--secrets FILE]
              [--module-global MODULE=FILE] [--module-user MODULE=FILE]
              [--code KEY=JSON]... [--env NAME=VALUE]... [--knob KEY=VALUE]... [--runtime KEY=VALUE]...
              [--connection KEY=VALUE]... [--authorized]
              resolve | dump | print-schema
```

The harness reads no ambient environment: `--env` is the whole environment. `--code` sets a code value; its value
is JSON (`--code demo.http.port=9000`, `--code 'demo.http.host="::1"'`). It writes JSON to stdout and exits 0, or
writes the error object and exits 2.

- `resolve`: `{"values": {KEY: {"value", "layer", "secret"}}, "attempts": [{"key", "outcome"}]}`. A secret's value is
  `"<redacted>"`. Keys that no layer sets are absent. `attempts` lists connection attempts sorted by key, and is
  present only when `--connection` is given.
- `dump`: the resolved configuration as a tree rooted at the application, secrets as `"<redacted>"`.
- `print-schema`: the master schema: the application object with one property per module.

## Cases

Each case is a directory in `cases/` holding `case.json` and the files its arguments name:

```json
{
  "description": "cli wins over env",
  "args": ["--app", "demo", "--schema", "{schema}/http.json", "--env", "DEMO_HTTP_PORT=8081", "--knob", "demo.http.port=9090"],
  "modes": { "secrets.yaml": "0600" },
  "command": "resolve",
  "expect": { "values": { "demo.http.port": { "value": 9090, "layer": "cli", "secret": false } } }
}
```

The suite copies the case directory to a scratch directory, sets each file in `modes` to its mode, replaces
`{schema}` with the path of `spec/schema/`, runs the harness there with `args` and `command`, and compares stdout as
JSON (key order and number formatting ignored) with `expect`, or with `expect_error` and exit status 2.
