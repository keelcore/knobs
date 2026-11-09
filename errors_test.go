package knobs

import (
	"errors"
	"strings"
	"testing"
)

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != code {
		t.Fatalf("want error code %s, got %v", code, err)
	}
}

func TestErrorsNeverCarrySecretValues(t *testing.T) {
	e := engine(t, dbSchema)
	e.Add(Defaults())
	e.Add(CLI([]string{"demo.db.password=hunter2"}))
	_, err := e.Resolve()
	wantCode(t, err, CodeSecretOnCLI)
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error carries the secret: %v", err)
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	*target = e
	return ok
}
