package cli

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestReadPromptLineLeavesFollowingInput(t *testing.T) {
	r := strings.NewReader("first\nsecond\n")
	one, err := readPromptLine(r)
	if err != nil || one != "first" {
		t.Fatalf("first=%q err=%v", one, err)
	}
	two, err := readPromptLine(r)
	if err != nil || two != "second" {
		t.Fatalf("second=%q err=%v", two, err)
	}
}

func TestReadPromptLineAcceptsFinalEOFData(t *testing.T) {
	got, err := readPromptLine(strings.NewReader("last line"))
	if err != nil || got != "last line" {
		t.Fatalf("line=%q err=%v", got, err)
	}
	if _, err := readPromptLine(strings.NewReader("")); err != io.EOF {
		t.Fatalf("empty EOF: %v", err)
	}
}

type noProgressReader struct{}

func (noProgressReader) Read([]byte) (int, error) { return 0, nil }

func TestReadPromptLineRejectsNoProgress(t *testing.T) {
	if _, err := readPromptLine(noProgressReader{}); err != io.ErrNoProgress {
		t.Fatalf("no progress: %v", err)
	}
}

func TestSafeWhoamiDropsUnexpectedFields(t *testing.T) {
	got, err := safeWhoami(json.RawMessage(`{"data":{"id":"user-id","email":"test@example.com","secret":"never"},"debug":"never"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got["id"] != "user-id" || got["email"] != "test@example.com" || len(got) != 2 {
		t.Fatalf("identity=%v", got)
	}
}
