package knobs

import (
	"fmt"
	"testing"
	"time"
)

func TestBindUnsetAndOverflow(t *testing.T) {
	e := engine(t, optionalSchema)
	e.Add(Defaults(), Runtime(map[string]string{"demo.opt.wait": "99999999999h"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	pinned := struct {
		Pin int64 `knobs:"opt.pin"`
	}{Pin: 9}
	if err := c.Bind(&pinned); err != nil || pinned.Pin != 9 {
		t.Fatalf("an unset key leaves its field: %v %d", err, pinned.Pin)
	}
	var waits struct {
		Wait time.Duration `knobs:"opt.wait"`
	}
	if err := c.Bind(&waits); err == nil {
		t.Fatal("a duration beyond time.Duration fails")
	}
}

type httpBinding struct {
	Port    int64         `knobs:"http.port"`
	Ratio   float64       `knobs:"http.ratio"`
	Debug   bool          `knobs:"http.debug"`
	Host    string        `knobs:"http.host"`
	Timeout time.Duration `knobs:"http.timeout"`
	Skip    string
}

type dbBinding struct {
	Password Secret `knobs:"db.password"`
}

func TestBind(t *testing.T) {
	e := engine(t, httpSchema, dbSchema)
	e.Add(Defaults())
	e.Add(Runtime(map[string]string{"demo.db.password": "hunter2"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var h httpBinding
	var d dbBinding
	if err := c.Bind(&h); err != nil {
		t.Fatal(err)
	}
	if err := c.Bind(&d); err != nil {
		t.Fatal(err)
	}
	if h.Port != 8080 || h.Ratio != 0.5 || h.Debug || h.Host != "127.0.0.1" || h.Timeout != 5*time.Second {
		t.Fatalf("bound %+v", h)
	}
	if d.Password.Reveal() != "hunter2" {
		t.Fatal("secret not bound")
	}
	for _, bad := range []any{
		h, nil, &struct {
			P string `knobs:"http.port"`
		}{}, &struct {
			P int64 `knobs:"http.nope"`
		}{}, &struct {
			P int64 `knobs:"nodot"`
		}{}, &struct {
			P []int `knobs:"http.port"`
		}{},
	} {
		if err := c.Bind(bad); err == nil {
			t.Fatalf("Bind(%T) must fail", bad)
		}
	}
}

func TestTableOf(t *testing.T) {
	rows, err := TableOf(&httpBinding{}, &dbBinding{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{
		{Key: "db.password", Type: "string", Secret: true},
		{Key: "http.debug", Type: "boolean"},
		{Key: "http.host", Type: "string"},
		{Key: "http.port", Type: "integer"},
		{Key: "http.ratio", Type: "number"},
		{Key: "http.timeout", Type: "duration"},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows %v\nwant %v", rows, want)
	}
	if _, err := TableOf(httpBinding{}); err == nil {
		t.Fatal("TableOf needs pointers to structs")
	}
	if _, err := TableOf(&struct {
		P []int `knobs:"m.p"`
	}{}); err == nil {
		t.Fatal("TableOf rejects unsupported field types")
	}
}
