package knobs

import (
	"runtime"
	"strconv"
	"sync"
	"testing"
)

const connSchema = `{
  "x-knobs-module": "http",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "port": {"type": "integer", "default": 8080, "minimum": 1, "maximum": 65535, "x-knobs-connection": true},
    "mode": {"type": "string", "default": "strict", "enum": ["strict", "lenient"], "x-knobs-connection": true},
    "host": {"type": "string", "default": "127.0.0.1"},
    "ratio": {"type": "number", "default": 0.5},
    "debug": {"type": "boolean", "default": false}
  }
}`

func processConfig(t testing.TB) *Config {
	t.Helper()
	e, err := New("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(Module{Schema: []byte(connSchema)}); err != nil {
		t.Fatal(err)
	}
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func connectionFor(i int) Connection {
	mode := "strict"
	if i%2 == 1 {
		mode = "lenient"
	}
	return Connection{
		Values:     map[string]string{"demo.http.port": strconv.Itoa(1 + i%65535), "demo.http.mode": mode},
		Authorized: true,
	}
}

// TestParallelConnections opens connections from many goroutines at once against one shared
// process configuration (run under -race): each sees only its own values, and the process
// configuration never changes.
func TestParallelConnections(t *testing.T) {
	c := processConfig(t)
	const workers, perWorker = 64, 200
	var wg sync.WaitGroup
	errs := make(chan string, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				i := w*perWorker + j
				r, attempts := c.WithConnection(connectionFor(i))
				port, _ := r.Int("http", "port")
				host, _ := r.String("http", "host")
				if port != int64(1+i%65535) || host != "127.0.0.1" || len(attempts) != 2 {
					errs <- "connection " + strconv.Itoa(i) + " saw another connection's values"
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if n, _ := c.Int("http", "port"); n != 8080 {
		t.Fatalf("the process configuration changed: %d", n)
	}
}

// maxBytesPerConnection bounds the heap one live connection holds for two overrides: its Config
// and its overrides, never a copy of the process configuration.
const maxBytesPerConnection = 512

// TestConnectionMemory holds 100, 1000 and 10000 connections alive at once and measures the heap
// they hold. A connection holds only its overrides, so the cost per connection is bounded and
// does not grow with the number of connections.
func TestConnectionMemory(t *testing.T) {
	c := processConfig(t)
	counts := []int{100, 1000, 10000}
	per := make([]float64, len(counts))
	for k, n := range counts {
		per[k] = minBytesPerConnection(c, n)
		t.Logf("%6d live connections: %7.0f bytes per connection", n, per[k])
		if per[k] > maxBytesPerConnection {
			t.Fatalf("%d connections: %.0f bytes per connection, over %d", n, per[k], maxBytesPerConnection)
		}
	}
	if per[2] > 2*per[1] {
		t.Fatalf("cost per connection grows with the count: %.0f at 1000, %.0f at 10000", per[1], per[2])
	}
}

// minBytesPerConnection is the least of three measurements: other allocations in the process only add
// to a reading, so the least is the connections' own cost.
func minBytesPerConnection(c *Config, n int) float64 {
	least := bytesPerConnection(c, n)
	for i := 0; i < 2; i++ {
		least = min(least, bytesPerConnection(c, n))
	}
	return least
}

// bytesPerConnection opens n connections in parallel, keeps them all alive, and returns the heap
// they hold divided by n.
func bytesPerConnection(c *Config, n int) float64 {
	held := make([]*Config, n)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			held[i], _ = c.WithConnection(connectionFor(i))
		}(i)
	}
	wg.Wait()
	runtime.GC()
	runtime.ReadMemStats(&after)
	used := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	runtime.KeepAlive(held)
	if used < 0 {
		used = 0
	}
	return float64(used) / float64(n)
}

func TestConnectionOverUnsetKey(t *testing.T) {
	e := engine(t, optionalSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	r, _ := c.WithConnection(Connection{Values: map[string]string{"demo.opt.limit": "3"}, Authorized: true})
	if vs := r.Values(); len(vs) != 1 || vs[0].Key != "demo.opt.limit" {
		t.Fatalf("a connection value with no process value is listed: %+v", vs)
	}
	// A connection built from a connection starts again from the process configuration.
	again, _ := r.WithConnection(Connection{})
	if _, ok := again.Get("demo.opt.limit"); ok {
		t.Fatal("connections do not stack")
	}
}

func TestWithConnection(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.http.port": "9000"}, Authorized: true})
	if n, _ := r.Int("http", "port"); n != 9000 || len(attempts) != 1 || attempts[0].Outcome != OutcomeApplied {
		t.Fatalf("connection: %d %+v", n, attempts)
	}
	if v, _ := r.Get("demo.http.port"); v.Layer != LayerConnection {
		t.Fatalf("layer %s", v.Layer)
	}
	if n, _ := c.Int("http", "port"); n != 8080 {
		t.Fatalf("WithConnection must not change the process config: %d", n)
	}
	for o, want := range map[Outcome]string{
		OutcomeApplied: "applied", OutcomeClamped: "clamped", OutcomeRejectedUnauthorized: "rejected_unauthorized",
		OutcomeRejectedNotOverridable: "rejected_not_overridable", OutcomeRejectedMalformed: "rejected_malformed",
		Outcome(99): "unknown",
	} {
		if o.String() != want {
			t.Fatalf("%d: %s", o, o.String())
		}
	}
}

// TestRuntimeTier: runtime beats cli, a connection beats runtime.
func TestConnectionOverRuntime(t *testing.T) {
	c := mustGate(t, CLI([]string{"demo.gate.window=40"}), Runtime(map[string]string{"demo.gate.window": "50"}))
	if n, _ := c.Int("gate", "window"); n != 50 || layerOf(t, c, "demo.gate.window") != LayerRuntime {
		t.Fatalf("runtime: %d", n)
	}
	r, _ := c.WithConnection(Connection{Values: map[string]string{"demo.gate.window": "60"}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 60 || layerOf(t, r, "demo.gate.window") != LayerConnection {
		t.Fatalf("connection: %d", n)
	}
}

// TestResolveNilSession, TestAbsentHeaderKeepsBase: no connection values leave the process
// configuration as it is, with no attempts.
func TestConnectionEmpty(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 16 || len(attempts) != 0 {
		t.Fatalf("got %d, %v", n, attempts)
	}
}

// TestHeaderOverlay, TestResolveWithHeaders: an authorized override on an allowlisted key wins.
func TestConnectionOverlay(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.gate.mode": "loose"}, Authorized: true})
	if s, _ := r.String("gate", "mode"); s != "loose" || attempts[0].Outcome != OutcomeApplied {
		t.Fatalf("got %q %v", s, attempts)
	}
}

// TestUnauthorizedCredentialIgnored: an allowlisted key is not overridden without authorization
// (fail-closed).
func TestConnectionUnauthorized(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.gate.window": "40"}})
	if n, _ := r.Int("gate", "window"); n != 16 || attempts[0].Outcome != OutcomeRejectedUnauthorized {
		t.Fatalf("got %d %v", n, attempts)
	}
}

// TestUnauthorizedHeaderIgnored: a key not marked overridable keeps its value, authorized or not.
func TestConnectionNotOverridable(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.gate.locked": "x", "demo.gate.api_key": "k"}, Authorized: true})
	if s, _ := r.String("gate", "locked"); s != "fixed" {
		t.Fatalf("got %q", s)
	}
	for _, a := range attempts {
		if a.Outcome != OutcomeRejectedNotOverridable {
			t.Fatalf("%s: %v", a.Key, a.Outcome)
		}
	}
}

// TestHeaderClampAndReject, TestHeaderAttempt: out of range clamps to the bound; a non-numeric
// value on a bounded key keeps the base; every attempt is recorded.
func TestConnectionClampAndReject(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{
		"demo.gate.window": "9999", "demo.gate.ratio": "0.2", "demo.gate.mode": "ok",
	}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 64 {
		t.Fatalf("clamped to max: %d", n)
	}
	if f, _ := r.Float("gate", "ratio"); f != 1 {
		t.Fatalf("clamped to min (only minimum set): %v", f)
	}
	want := []Outcome{OutcomeApplied, OutcomeClamped, OutcomeClamped}
	for i, a := range attempts {
		if a.Outcome != want[i] {
			t.Fatalf("%s: %v, want %v", a.Key, a.Outcome, want[i])
		}
	}
	r, attempts = c.WithConnection(Connection{Values: map[string]string{"demo.gate.window": "abc"}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 16 || attempts[0].Outcome != OutcomeRejectedMalformed {
		t.Fatalf("malformed keeps the base: %d %v", n, attempts)
	}
}

// FuzzConnection: any connection key and text yield exactly one attempt, and the process
// configuration is unchanged.
func FuzzConnection(f *testing.F) {
	f.Add("demo.http.port", "9000", true)
	f.Add("demo.db.password", "x", true)
	f.Add("nope", "", false)
	e := engine(f, httpSchema, dbSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, key, text string, authorized bool) {
		_, attempts := c.WithConnection(Connection{Values: map[string]string{key: text}, Authorized: authorized})
		if len(attempts) != 1 {
			t.Fatalf("attempts %v", attempts)
		}
		if n, _ := c.Int("http", "port"); n != 8080 {
			t.Fatalf("the process configuration changed: %d", n)
		}
	})
}

func TestConnectionBoundBreakingARuleIsMalformed(t *testing.T) {
	e := engine(t, nestedSchema)
	e.Add(Code(map[string]any{"demo.svc.db.tls.ca": "x"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	_, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.svc.step": "12"}, Authorized: true})
	if attempts[0].Outcome != OutcomeRejectedMalformed {
		t.Fatalf("clamped to an exclusive maximum: %v", attempts[0].Outcome)
	}
}
