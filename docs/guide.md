# knobs guide

How an application configures itself with knobs, in Go. The contract every language follows is
[spec/README.md](../spec/README.md); this guide shows it from an application's side.

## 1. A module owns its knobs

Each Go package that has settings describes them in a JSON Schema and exports a `knobs.Module`:

```go
// package httpd

//go:embed knobs.json
var schema []byte

var Knobs = knobs.Module{
    Schema:     schema,
    Code:       map[string]any{"workers": runtime.NumCPU()}, // the code layer: values set in code
    GlobalPath: "/etc/demo/http.yaml",                       // optional: this module's own files
}
```

`knobs.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "x-knobs-module": "http",
  "x-knobs-owner": "example.com/demo",
  "type": "object",
  "additionalProperties": false,
  "required": ["port"],
  "properties": {
    "port": { "type": "integer", "default": 8080, "minimum": 1, "maximum": 65535, "x-knobs-connection": true },
    "workers": { "type": "integer", "minimum": 1 },
    "read_timeout": { "type": "string", "x-knobs-type": "duration", "default": "5s" },
    "api_key": { "type": "string", "x-knobs-secret": true }
  }
}
```

The schema keywords knobs accepts are listed in the spec; any other keyword fails `Attach`, so every rule in a schema
is enforced.

## 2. main attaches the modules

```go
func main() {
    engine, err := knobs.New("demo")
    if err == nil {
        err = engine.Attach(httpd.Knobs, store.Knobs)
    }
    if err == nil {
        err = engine.AddStandard(knobs.Standard{Knobs: knobFlags}) // knobs reads the environment itself
    }
    var config *knobs.Config
    if err == nil {
        config, err = engine.Resolve()
    }
    if err != nil {
        log.Fatal(err)
    }
    run(config)
}
```

`Attach` takes any number of modules, in one call or several; two modules with one name fail with `module_conflict`.
`AddStandard` adds every standard layer, and `Resolve` validates each one before it is merged.

## 3. Where values come from

Lowest to highest precedence:

| Layer | Source |
| --- | --- |
| `code` | each module's `Code`, then any `knobs.Code` layer |
| `schema` | each property's `default` |
| `global` | `/etc/demo/config.yaml` (Windows: `%ProgramData%\demo\config.yaml`; the file name is a knob), then each module's `GlobalPath` |
| `user` | `$XDG_CONFIG_HOME/demo/config.yaml` (else `~/.config/demo/config.yaml`; Windows: `%APPDATA%\demo\config.yaml`), then each module's `UserPath` |
| `secrets` | the secrets file, when one is named: its owner's alone (mode bits on Linux and macOS, owner and DACL on Windows) |
| `env` | `DEMO_HTTP_PORT`, or the property's `x-knobs-env` |
| `cli` | `--knob demo.http.port=9000` |
| `runtime` | `Standard.Runtime`, values the program sets while running |

The application's file holds every module under its root key:

```yaml
demo:
  http:
    port: 9000
  store:
    url: postgres://db.internal/app
```

A module's own file holds its mapping directly (`port: 9001`) and overrides the application's file within the same
layer. The file paths themselves are knobs of the reserved module `knobs`, set by cli or env only:
`--knob demo.knobs.global_path=...`, `DEMO_KNOBS_USER_PATH=...`, `DEMO_KNOBS_SECRETS_PATH=...`. The global and user
files share one file name, `config.yaml` unless the knob `demo.knobs.file_name` names another
(`--knob demo.knobs.file_name=demo.yaml`, `DEMO_KNOBS_FILE_NAME=demo.yaml`); a full path outranks it. knobs reads
the process environment itself; `Standard.Env` replaces it, for tests and embedding.

## 4. Reading values

```go
port, err := config.Int("http", "port")
timeout, err := config.Duration("http", "read_timeout")
key, err := config.Secret("http", "api_key") // key.Reveal() is the one deliberate read
```

Or bind a struct:

```go
type HTTP struct {
    Port        int64         `knobs:"http.port"`
    ReadTimeout time.Duration `knobs:"http.read_timeout"`
    APIKey      knobs.Secret  `knobs:"http.api_key"`
}
var h HTTP
err := config.Bind(&h)
```

`config.Dump()` is the whole configuration in the application's file form, secrets as `<redacted>`;
`config.Values()` lists every value with the layer that set it.

## 5. Secrets

A value from the secrets file, or for a key marked `x-knobs-secret`, is secret. It prints as `<redacted>` in every
dump, error and printed form; `knobs.Secret` redacts itself for every `fmt` verb and in JSON. A key marked secret is
refused in the global and user files (`secret_in_plain_file`), on the command line (`secret_on_cli`) and in code
(`secret_in_code`); it comes from the secrets file, env or runtime.

## 6. Connections

A caller's per-connection state overrides the process configuration for that connection only:

```go
conn, attempts := config.WithConnection(knobs.Connection{
    Values:     map[string]string{"demo.http.port": fromCaller},
    Authorized: callerAuthorized,
})
```

The application maps whatever the caller sends (connection headers, call metadata) to keys. Only keys marked
`x-knobs-connection` change, only when `Authorized`, numbers clamped to the schema's bounds; every value is recorded in
`attempts` with its outcome. A connection holds only its own overrides over the shared process configuration.

## 7. In the application's tests

```go
func TestKnobs(t *testing.T) {
    knobstest.Schema(t, buildEngine(t), "knobs-schema.json") // the committed master schema is current
    knobstest.SingleMajor(t)                                 // one major version of knobs in the binary
}
```

`KNOBS_WRITE_SCHEMA=1 go test` rewrites the committed master schema; every knob change then shows in review.

## 8. Versioning the configuration surface

`knobs-schema-bump OLD NEW` compares two master schemas: a removed field is major, an added field minor, no change
patch. It prints the release hook's two lines, so a `create-release.local.sh` can call
`go run github.com/keelcore/knobs/cmd/knobs-schema-bump@vX.Y.Z` with the schema at the previous tag and at `HEAD`.
