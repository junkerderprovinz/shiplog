package updates

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

// restartedDuring is what a run that was going when ShipLog stopped says.
const restartedDuring = "ShipLog restarted during the run"

type kept struct {
	ID         string   `json:"id"`
	State      string   `json:"state"`
	Containers []result `json:"containers"`
	Error      string   `json:"error,omitempty"`
}

// Restore reads the runs File kept. A run that was going when ShipLog stopped
// failed, since nothing carries it on.
func (r *Runs) Restore() error {
	if r.File == "" {
		return nil
	}
	raw, err := os.ReadFile(r.File)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var runs []kept
	if err := json.Unmarshal(raw, &runs); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range runs {
		ru := &run{ID: k.ID, State: k.State, Containers: k.Containers, Error: k.Error}
		if ru.Containers == nil {
			ru.Containers = []result{}
		}
		if ru.State == "running" {
			ru.State, ru.Error = "failed", restartedDuring
		}
		r.add(ru)
	}
	return nil
}

// add holds ru as the newest run and lets the oldest go beyond keep. The
// caller holds mu.
func (r *Runs) add(ru *run) {
	if r.runs == nil {
		r.runs = map[string]*run{}
	}
	r.runs[ru.ID] = ru
	r.order = append(r.order, ru.ID)
	if len(r.order) > keep {
		delete(r.runs, r.order[0])
		r.order = r.order[1:]
	}
}

// save replaces File with the runs held, so a crash leaves the old file or
// the new one. The caller holds mu.
func (r *Runs) save() {
	if r.File == "" {
		return
	}
	runs := make([]kept, 0, len(r.order))
	for _, id := range r.order {
		ru := r.runs[id]
		runs = append(runs, kept{ID: id, State: ru.State, Containers: ru.Containers, Error: ru.Error})
	}
	raw, _ := json.Marshal(runs)
	if err := write(r.File, raw); err != nil {
		log.Printf("shiplog: could not keep the update runs in %s: %v", r.File, err)
	}
}

func write(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".runs-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
