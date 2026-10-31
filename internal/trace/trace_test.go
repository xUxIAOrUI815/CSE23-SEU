package trace

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRecordReadBack(t *testing.T) {
	var buf bytes.Buffer
	r := NewRecorder(&buf)
	if err := r.Record(Event{Type: "exec", Detail: "python main.py"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Record(Event{Type: "exit", Detail: "0"}); err != nil {
		t.Fatal(err)
	}

	var evs []Event
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		evs = append(evs, ev)
	}
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}
	if evs[0].Type != "exec" {
		t.Errorf("evs[0].Type = %q, want exec", evs[0].Type)
	}
}
