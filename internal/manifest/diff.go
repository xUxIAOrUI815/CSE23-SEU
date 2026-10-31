package manifest

import "sort"

// Change 描述 a → b 的一个字段变化；Old/New 为 nil 表示新增/删除。
type Change struct {
	Field string
	Old   any
	New   any
}

// Diff 逐字段比较两个 manifest，返回按字段名排序的变化列表。
func Diff(a, b *Manifest) []Change {
	var changes []Change
	add := func(field string, old, new any) {
		changes = append(changes, Change{Field: field, Old: old, New: new})
	}

	if a.Name != b.Name {
		add("name", a.Name, b.Name)
	}
	if a.Version != b.Version {
		add("version", a.Version, b.Version)
	}
	if !equalStrings(a.Command, b.Command) {
		add("command", a.Command, b.Command)
	}
	if !equalStringMap(a.Env, b.Env) {
		add("env", a.Env, b.Env)
	}
	if !equalStrings(a.Deps, b.Deps) {
		add("deps", a.Deps, b.Deps)
	}
	if a.Prompt != b.Prompt {
		add("prompt", a.Prompt, b.Prompt)
	}
	if !equalModel(a.Model, b.Model) {
		add("model", a.Model, b.Model)
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func equalModel(a, b *ModelSpec) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.ID != b.ID {
		return false
	}
	if !equalFloatPtr(a.Temperature, b.Temperature) {
		return false
	}
	if !equalIntPtr(a.Seed, b.Seed) {
		return false
	}
	return true
}

func equalFloatPtr(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalIntPtr(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
