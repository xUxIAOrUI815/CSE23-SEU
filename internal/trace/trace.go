package trace

import (
	"encoding/json"
	"io"
	"time"
)

type Event struct {
	Time   time.Time `json:"time"`
	Type   string    `json:"type"` // "exec" / "exit" / "output"
	Detail string    `json:"detail,omitempty"`
}

type Recorder struct {
	w io.Writer
}

// NewRecorder 创建一个 JSONL 记录器（每行一个 Event）。
func NewRecorder(w io.Writer) *Recorder {
	return &Recorder{w: w}
}

func (r *Recorder) Record(ev Event) error {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	return json.NewEncoder(r.w).Encode(ev)
}
