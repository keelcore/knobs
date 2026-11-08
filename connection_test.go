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
	if err := e.Attach([]byte(connSchema), "", ""); err != nil {
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
