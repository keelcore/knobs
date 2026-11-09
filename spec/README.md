# knobs specification

This directory is the contract every implementation follows. A behavior is specified when a case in `cases/` shows
it; `tests/conformance.bats` runs every case against every implementation.

## Names

An application is named once (`app`); a schema names its module (`x-knobs-module`); each property is a knob. Names
are lowercase snake case (`^[a-z][a-z0-9_]*$`). The module name `knobs` is reserved for the engine's own settings.

| Form | Pattern | Example (app `demo`, module `http`, knob `read_timeout`) |
| --- | --- | --- |
| key | `<app>.<module>.<name>` (a nested knob's name is its dotted path) | `demo.http.read_timeout` |
| file | `<app>:` → `<module>:` → `<name>:` (a nested knob under each of its objects) | `demo:` → `http:` → `read_timeout:` |
| env | `<APP>_<MODULE>_<NAME>`, each dot of a nested name `_`, or the property's `x-knobs-env` | `DEMO_HTTP_READ_TIMEOUT` |
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

Schemas are validated, and every value is validated against them, by a full JSON Schema validator: every keyword of
the schema's dialect is enforced, as that dialect defines it (`format` is an annotation in 2019-09 and 2020-12, and is
not asserted). A schema is JSON or YAML, in the dialect its `$schema` names (2019-09 or 2020-12; 2020-12 when it
names none). A schema that is not valid against its dialect's metaschema fails with `schema_invalid`, as does a `$ref`
to anything outside the schema: no schema reads a file or the network.

**Unrecognized keywords fail.** A validator ignores a keyword it does not recognize, so a misspelled rule (`patern`,
`minimun`) would go unenforced. Every schema object, at every depth, may hold only its dialect's keywords and the
`x-knobs-` keywords: the schema is checked against its dialect's metaschema with `"unevaluatedProperties": false`,
which the metaschema's dynamic reference carries into every subschema. A schema holding any other keyword fails with
`schema_invalid`, and the error lists every such keyword with the location of the schema object holding it, as a
JSON Pointer fragment (`"patern"` at `#/properties/name`). An unrecognized keyword also leaves the keywords of every
schema object above it unevaluated, up to the root; those are not listed: a keyword is listed when no unrecognized
keyword lies deeper inside the object holding it. Drafts 4, 6 and 7 fail with `schema_invalid`: their metaschemas
reach subschemas by a plain `$ref`, which the check cannot follow.

A schema with `x-knobs-module` is one module. A schema without it is an **application schema**: each of its properties
is a module, named by the property (`listeners` in an application schema is the module `listeners`).

- A property with `properties` of its own is a **nested object**; any other property is a **knob**, whatever its type
  (`string`, `integer`, `number`, `boolean`, `array`, `object`, or none). A knob in a nested object is named by its
  dotted path inside the module: `listeners` → `http` → `port` is the knob `http.port`, key `demo.listeners.http.port`,
  env `DEMO_LISTENERS_HTTP_PORT`.
- A module and each nested object is `"type": "object"` with `"additionalProperties": false`, so a key no schema
  defines fails in every layer; `required` names only properties it defines.
- The `x-knobs-` keywords: `x-knobs-module` at the top of a module schema; `x-knobs-owner` there or on a module;
  `x-knobs-type` (`duration`), `x-knobs-secret`, `x-knobs-connection` and `x-knobs-env` on a knob. Any other
  `x-knobs-` keyword, or one out of place, fails with `schema_invalid`.
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
  which holds that module's mapping directly, and an application schema's own file, which holds the mapping of its
  modules directly. Values keep their YAML types: `port: "8080"` is a string. A file missing
  at its default location is skipped; a file named explicitly that is missing fails with `file_not_found`.
  A module's own files are optional: missing, they are skipped. An `integer` property accepts a YAML number with no
  fractional part; a `number` property accepts any YAML number.
- **The secrets file** is its owner's alone, or it fails with `secrets_file_permissions`. Where file modes carry access
  rights (Linux, macOS), no group or other bit is set. On Windows, where they do not, its owner is the current user,
  SYSTEM or Administrators, and its DACL grants access to those three only.
- **env** reads only the variables named by the schemas; an empty value counts as unset.
- **code** values are typed, as files are: `9000` is an integer, `"9000"` a string. A schema `default` overrides a code
  value for the same key; a code value is the baseline for a key whose schema gives no default. A module's own code
  values are keyed by its knob names; an application schema's by `<module>.<name>`.
- **env, cli, runtime and connection** values are text, converted to the property's type: `integer` is optional `-` and
  digits; `number` is a decimal number; `boolean` is `true` or `false`; `string` is the text as given; `array` is a
  JSON list, or comma-separated items, each trimmed, empty ones dropped, each converted to the item type; `object` is
  a JSON object; a knob with no single type takes the text as JSON when it parses, else as a string.
- **Every layer is validated** against the schemas before it is merged, so an error names its layer and key: each
  value against its knob's schema.
- **Each module is validated whole** after the process layers (all but `connection`) merge, its set values as one
  object, so the rules that span keys (`required`, `dependentRequired`, `if`/`then`) hold; the error's layer is
  `resolve`. A key that `required`, `dependentRequired` or `dependencies` wants and no layer sets fails with
  `missing_required`, on that key.
- **Error codes** of a value that breaks its schema: `type` is `invalid_type`; `enum` and `const` `not_in_enum`;
  `minimum`, `maximum` and their exclusive forms `out_of_range`; `minLength`, `maxLength`, `minItems`, `maxItems`,
  `minProperties` and `maxProperties` `invalid_length`; any other keyword (`pattern`, `multipleOf`, `uniqueItems`,
  `not`, `anyOf`, ...) `invalid_value`. A value that breaks several takes the first of that order.

## Connections

The layers from `code` to `runtime` are the process configuration, resolved once. A connection carries the
caller's state: a set of key-to-text values, and whether the connection is authorized for overrides. The caller
sends that state with every connection (as connection headers, call metadata or anything else); the application
maps it to values, and knobs never sees the transport. knobs keeps no state between connections: the same values and
authorization give the same result on any node. Each value is checked in this order:

1. Not authorized: `rejected_unauthorized`.
2. No schema defines the key, it is not `x-knobs-connection`, or it is secret: `rejected_not_overridable`.
3. The text does not convert to the property's type, or breaks any rule of its schema but `minimum`/`maximum`:
   `rejected_malformed`.
4. A number outside `minimum`/`maximum` is set to the nearer bound: `clamped`; when the bound itself breaks a rule
   (an exclusive bound, `multipleOf`), `rejected_malformed`.
5. Otherwise: `applied`.

## Errors

An error is reported as `{"error": {"code", "layer", "key", "keywords"}}`; `key` is the dotted key when the error
concerns one, and is absent otherwise; `keywords`, present only when a schema fails for unrecognized keywords, lists
each as `{"keyword", "location"}`, in location order. Codes: `schema_invalid`, `module_conflict`, `file_not_found`,
`file_parse`, `unknown_key`, `invalid_type`, `out_of_range`, `not_in_enum`, `invalid_length`, `invalid_value`,
`secret_in_plain_file`, `secret_on_cli`, `secrets_file_permissions`, `secret_in_code`, `missing_required`,
`invalid_argument`. Messages are free text, never part of the contract, and never contain a secret value. When several
errors occur, the first is reported: layers in precedence order, keys in sorted order within a layer.

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
writes the error object and exits 2. `--module-global` and `--module-user` name a schema's own files by its module, or by
the application for an application schema.

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
