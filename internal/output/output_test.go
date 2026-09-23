package output

import (
	"errors"
	"strings"
	"testing"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestJSONToEncodesIndentedJSON(t *testing.T) {
	var got strings.Builder
	if err := JSONTo(&got, map[string]string{"result": "ok"}); err != nil {
		t.Fatalf("JSONTo() error = %v", err)
	}
	if want := "{\n  \"result\": \"ok\"\n}\n"; got.String() != want {
		t.Errorf("JSONTo() = %q, want %q", got.String(), want)
	}
}

func TestJSONToReturnsWriterError(t *testing.T) {
	want := errors.New("output device unavailable")
	if err := JSONTo(failingWriter{err: want}, map[string]string{"result": "ok"}); !errors.Is(err, want) {
		t.Fatalf("JSONTo() error = %v, want writer error %v", err, want)
	}
}
