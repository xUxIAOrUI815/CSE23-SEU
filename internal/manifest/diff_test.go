package manifest

import "testing"

func TestDiffChanged(t *testing.T) {
	a := &Manifest{Name: "v1", Command: []string{"echo"}, Prompt: "A"}
	b := &Manifest{Name: "v1", Command: []string{"echo"}, Prompt: "B"}
	changes := Diff(a, b)
	if len(changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1", len(changes))
	}
	c := changes[0]
	if c.Field != "prompt" || c.Old != "A" || c.New != "B" {
		t.Errorf("change = %+v, want prompt A->B", c)
	}
}

func TestDiffAddedRemoved(t *testing.T) {
	a := &Manifest{Name: "v1", Command: []string{"echo"}, Env: map[string]string{"K": "V"}}
	b := &Manifest{Name: "v1", Command: []string{"echo"}, Deps: []string{"x"}}
	changes := Diff(a, b)
	if len(changes) != 2 {
		t.Fatalf("len(changes) = %d, want 2", len(changes))
	}
	fields := map[string]bool{}
	for _, c := range changes {
		fields[c.Field] = true
	}
	if !fields["env"] || !fields["deps"] {
		t.Errorf("expected env and deps changes, got %+v", changes)
	}
}
