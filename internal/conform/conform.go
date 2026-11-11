// Package conform is the Go conformance harness, knobs-conform (spec/README.md, The conformance
// harness): arguments in, JSON out, exit status 0 or 2. It reads no ambient environment.
package conform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/keelcore/knobs"
	"go.yaml.in/yaml/v3"
)

// invocation is one parsed command line.
type invocation struct {
	app                      string
	schemas                  []string
	global, user, secrets    string
	moduleGlobal, moduleUser map[string]string
	code                     map[string]any
	env                      map[string]string
	knobs, args              []string
	runtime                  map[string]string
	connection               map[string]string
	authorized, standard     bool
	command                  string
}

// Main runs the harness and returns its exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	out, err := run(args)
	if err != nil {
		var ke *knobs.Error
		if !errors.As(err, &ke) {
			ke = &knobs.Error{Code: knobs.CodeInvalidArgument, Layer: "args", Msg: err.Error()}
		}
		obj := map[string]any{"code": ke.Code, "layer": ke.Layer}
		if ke.Key != "" {
			obj["key"] = ke.Key
		}
		if len(ke.Keywords) > 0 {
			list := make([]map[string]string, len(ke.Keywords))
			for i, k := range ke.Keywords {
				list[i] = map[string]string{"keyword": k.Name, "location": k.Location}
			}
			obj["keywords"] = list
		}
		out = map[string]any{"error": obj}
		_, _ = fmt.Fprintln(stderr, ke.Error())
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if encErr := enc.Encode(out); encErr != nil || err != nil {
		return 2
	}
	return 0
}

func run(args []string) (any, error) {
	inv, err := parse(args)
	if err != nil {
		return nil, err
	}
	if inv.command == "types" {
		return knobs.TableOf(&httpBinding{}, &dbBinding{})
	}
	e, err := build(inv)
	if err != nil {
		return nil, err
	}
	if inv.command == "print-schema" {
		return e.Schema(), nil
	}
	c, err := e.Resolve()
	if err != nil {
		return nil, err
	}
	if inv.command == "dump" {
		return c.Dump(), nil
	}
	return resolveOutput(c, inv), nil
}

func resolveOutput(c *knobs.Config, inv *invocation) map[string]any {
	out := map[string]any{}
	if inv.connection != nil {
		var attempts []knobs.Attempt
		c, attempts = c.WithConnection(knobs.Connection{Values: inv.connection, Authorized: inv.authorized})
		list := make([]map[string]any, 0, len(attempts))
		for _, a := range attempts {
			list = append(list, map[string]any{"key": a.Key, "outcome": a.Outcome.String()})
		}
		out["attempts"] = list
	}
	values := map[string]any{}
	for _, v := range c.Values() {
		values[v.Key] = map[string]any{"value": v.Display(), "layer": v.Layer, "secret": v.Secret}
	}
	out["values"] = values
	return out
}

// build attaches the schemas and adds the layers the arguments name, in precedence order.
func build(inv *invocation) (*knobs.Engine, error) {
	e, err := knobs.New(inv.app)
	if err != nil {
		return nil, err
	}
	for _, path := range inv.schemas {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, &knobs.Error{Code: knobs.CodeFileNotFound, Layer: "schema", Msg: err.Error()}
		}
		mod := moduleOf(data, inv.app)
		if err := e.Attach(knobs.Module{Schema: data, GlobalPath: inv.moduleGlobal[mod], UserPath: inv.moduleUser[mod]}); err != nil {
			return nil, err
		}
	}
	env := func(name string) (string, bool) { v, ok := inv.env[name]; return v, ok }
	if inv.standard {
		// The standard layers, as a program gets them: --env is its whole environment, and the files come
		// from the given paths, else the path and file-name knobs, else the default locations.
		if len(inv.code) > 0 {
			return nil, errors.New("--code does not combine with --standard")
		}
		return e, e.AddStandard(knobs.Standard{GlobalPath: inv.global, UserPath: inv.user, SecretsPath: inv.secrets,
			Env: env, Args: inv.args, Knobs: inv.knobs, Runtime: inv.runtime})
	}
	layers := []knobs.Layer{
		knobs.Code(inv.code),
		knobs.Defaults(),
		knobs.File(knobs.LayerGlobal, inv.global, true),
		knobs.File(knobs.LayerUser, inv.user, true),
		knobs.File(knobs.LayerSecrets, inv.secrets, true),
		knobs.Env(env),
		knobs.CLI(inv.knobs),
		knobs.Runtime(inv.runtime),
	}
	return e, e.Add(layers...)
}

// moduleOf names a schema's own files: its x-knobs-module, or the application for an application
// schema, which has none. Attach checks the rest.
func moduleOf(data []byte, app string) string {
	var s struct {
		Module *string `yaml:"x-knobs-module"`
	}
	if yaml.Unmarshal(data, &s) != nil || s.Module == nil {
		return app
	}
	return *s.Module
}

func parse(args []string) (*invocation, error) {
	inv := &invocation{moduleGlobal: map[string]string{}, moduleUser: map[string]string{}, env: map[string]string{},
		runtime: map[string]string{}, code: map[string]any{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			if i != len(args)-1 {
				return nil, fmt.Errorf("the command %q comes last", a)
			}
			inv.command = a
			continue
		}
		if a == "--standard" {
			inv.standard = true
			continue
		}
		if a == "--authorized" {
			inv.authorized = true
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("%s needs a value", a)
		}
		i++
		if err := inv.set(a, args[i]); err != nil {
			return nil, err
		}
	}
	return inv, inv.check()
}

func (inv *invocation) set(flag, v string) error {
	switch flag {
	case "--app":
		inv.app = v
	case "--schema":
		inv.schemas = append(inv.schemas, v)
	case "--global":
		inv.global = v
	case "--user":
		inv.user = v
	case "--secrets":
		inv.secrets = v
	case "--arg":
		inv.args = append(inv.args, v)
	case "--knob":
		inv.knobs = append(inv.knobs, v)
	case "--module-global":
		return pair(inv.moduleGlobal, flag, v)
	case "--module-user":
		return pair(inv.moduleUser, flag, v)
	case "--env":
		return pair(inv.env, flag, v)
	case "--runtime":
		return pair(inv.runtime, flag, v)
	case "--code":
		return inv.setCode(v)
	case "--connection":
		if inv.connection == nil {
			inv.connection = map[string]string{}
		}
		return pair(inv.connection, flag, v)
	default:
		return fmt.Errorf("unknown flag %s", flag)
	}
	return nil
}

// setCode reads --code KEY=JSON.
func (inv *invocation) setCode(v string) error {
	k, raw, ok := strings.Cut(v, "=")
	var val any
	if !ok || k == "" || json.Unmarshal([]byte(raw), &val) != nil {
		return errors.New("--code wants KEY=JSON")
	}
	inv.code[k] = val
	return nil
}

func pair(m map[string]string, flag, v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok || k == "" {
		return fmt.Errorf("%s wants NAME=VALUE", flag)
	}
	m[k] = val
	return nil
}

func (inv *invocation) check() error {
	switch inv.command {
	case "types":
		return nil
	case "resolve", "dump", "print-schema":
		if inv.app == "" {
			return errors.New("--app is required")
		}
		return nil
	case "":
		return errors.New("no command: resolve, dump, print-schema or types")
	}
	return fmt.Errorf("unknown command %q", inv.command)
}
