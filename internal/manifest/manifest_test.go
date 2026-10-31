package manifest

import "testing"

func TestParseValid(t *testing.T) {
	m, err := Parse([]byte(`{"name":"v1","version":"1","command":["python","main.py"]}`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.Name != "v1" {
		t.Errorf("Name = %q, want %q", m.Name, "v1")
	}
	if len(m.Command) != 2 {
		t.Fatalf("len(Command) = %d, want 2", len(m.Command))
	}
}

func TestParseMissingCommand(t *testing.T) {
	_, err := Parse([]byte(`{"name":"v1"}`))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestRoundTrip(t *testing.T) {
	in := &Manifest{Name: "v1", Version: "1", Command: []string{"python", "main.py"}}
	data, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != in.Name || len(m.Command) != len(in.Command) {
		t.Errorf("round trip mismatch: got %+v", m)
	}
}
