package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cryolab/internal/manifest"
)

type Store struct {
	Root string // 已解析为绝对路径
}

// Open 确保 <root>/snapshots 目录存在并返回 Store。
func Open(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	s := &Store{Root: abs}
	if err := os.MkdirAll(filepath.Join(abs, "snapshots"), 0o755); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) snapshotDir(name string) string {
	return filepath.Join(s.Root, "snapshots", name)
}

// Create 建 <root>/snapshots/<name>/{manifest.json, base/}。
func (s *Store) Create(name string, m *manifest.Manifest) error {
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || name == "" {
		return fmt.Errorf("store: invalid snapshot name %q", name)
	}
	dir := s.snapshotDir(name)
	if err := os.MkdirAll(filepath.Join(dir, "base"), 0o755); err != nil {
		return err
	}
	data, err := m.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
}

// List 返回快照名（字典序）。
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "snapshots"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// LoadManifest 读回某个快照的 manifest。
func (s *Store) LoadManifest(name string) (*manifest.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(s.snapshotDir(name), "manifest.json"))
	if err != nil {
		return nil, err
	}
	return manifest.Parse(data)
}

// BasePath 返回某个快照的冻结只读层目录。
func (s *Store) BasePath(name string) string {
	return filepath.Join(s.snapshotDir(name), "base")
}
