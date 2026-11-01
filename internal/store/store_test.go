package store

import (
	"os"
	"testing"

	"cryolab/internal/manifest"
)

func TestCreateAndList(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Create("v1", &manifest.Manifest{Name: "v1", Command: []string{"echo"}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	names, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 1 || names[0] != "v1" {
		t.Fatalf("names = %v, want [v1]", names)
	}
	base, err := s.BasePath("v1")
	if err != nil {
		t.Fatalf("BasePath: %v", err)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("BasePath not a dir: %v", err)
	}
	m, err := s.LoadManifest("v1")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "v1" {
		t.Fatalf("LoadManifest name = %q, want v1", m.Name)
	}
}

func TestCreateRejectsUnsafeName(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Create("../evil", &manifest.Manifest{Name: "v1", Command: []string{"echo"}}); err == nil {
		t.Fatal("expected error for unsafe name")
	}
}

func TestLoadManifestRejectsUnsafeName(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := s.LoadManifest("../evil"); err == nil {
		t.Fatal("expected error for unsafe name")
	}
}

func TestBasePathRejectsUnsafeName(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := s.BasePath("../evil"); err == nil {
		t.Fatal("expected error for unsafe name")
	}
}

func TestCreateRejectsExisting(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m := &manifest.Manifest{Name: "v1", Command: []string{"echo"}}
	if err := s.Create("v1", m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create("v1", m); err == nil {
		t.Fatal("expected error for existing snapshot name")
	}
}
