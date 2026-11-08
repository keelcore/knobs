package conform

import (
	"time"

	"github.com/keelcore/knobs"
)

// httpBinding and dbBinding are the Go bindings of spec/schema/http.json and db.json; the
// type-match audit (scripts/test/check-schema-types.sh) compares their table (`types`) with the
// schemas.
type httpBinding struct {
	Port        int64         `knobs:"http.port"`
	Host        string        `knobs:"http.host"`
	ReadTimeout time.Duration `knobs:"http.read_timeout"`
	Mode        string        `knobs:"http.mode"`
	Ratio       float64       `knobs:"http.ratio"`
	Debug       bool          `knobs:"http.debug"`
}

type dbBinding struct {
	URL      string       `knobs:"db.url"`
	Password knobs.Secret `knobs:"db.password"`
	Pool     int64        `knobs:"db.pool"`
}
