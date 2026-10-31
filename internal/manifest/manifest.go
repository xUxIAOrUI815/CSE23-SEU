package manifest

import (
	"encoding/json"
	"fmt"
)

type ModelSpec struct {
	ID          string   `json:"id"`
	Temperature *float64 `json:"temperature,omitempty"`
	Seed        *int64   `json:"seed,omitempty"`
}

type Manifest struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	Deps    []string          `json:"deps,omitempty"`
	Prompt  string            `json:"prompt,omitempty"`
	Model   *ModelSpec        `json:"model,omitempty"`
}

// Parse 反序列化并校验必填字段 name 与 command。
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest: missing required field %q", "name")
	}
	if len(m.Command) == 0 {
		return nil, fmt.Errorf("manifest: missing required field %q", "command")
	}
	return &m, nil
}

// Marshal 以两空格缩进序列化。
func (m *Manifest) Marshal() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
