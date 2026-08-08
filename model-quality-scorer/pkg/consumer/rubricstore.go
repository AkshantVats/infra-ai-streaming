// SPDX-License-Identifier: MIT

package consumer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
)

// RubricStore resolves the shared rubric template for a task_type and
// version. A single RubricStore instance is shared across every judge
// call in a batch (DESIGN.md's "shared rubric template") rather than
// re-reading and re-validating the same file per sample.
type RubricStore interface {
	Get(taskType string, version int) (rubric.JudgeRubric, error)
}

// MapRubricStore is an in-memory RubricStore, primarily for tests and
// for callers that already have rubrics loaded (e.g. from a config
// service) rather than files on disk.
type MapRubricStore struct {
	rubrics map[string]rubric.JudgeRubric
}

// NewMapRubricStore builds a MapRubricStore from rubrics already
// validated by the caller.
func NewMapRubricStore(rubrics []rubric.JudgeRubric) *MapRubricStore {
	m := make(map[string]rubric.JudgeRubric, len(rubrics))
	for _, r := range rubrics {
		m[rubricKey(r.TaskType, r.Version)] = r
	}
	return &MapRubricStore{rubrics: m}
}

func (s *MapRubricStore) Get(taskType string, version int) (rubric.JudgeRubric, error) {
	r, ok := s.rubrics[rubricKey(taskType, version)]
	if !ok {
		return rubric.JudgeRubric{}, fmt.Errorf("rubric store: no rubric for task_type %q version %d", taskType, version)
	}
	return r, nil
}

// FileRubricStore resolves rubrics from JSON template files under a
// directory (see rubrics/ in this module for the shared templates
// shipped by default), named "<task_type>.v<version>.json". A rubric
// that fails validation on load is not cached — every message that
// references it is routed to the DLQ (dlq.ReasonMalformedRubric) until
// the template file is fixed and the store is restarted or evicted.
type FileRubricStore struct {
	dir   string
	mu    sync.RWMutex
	cache map[string]rubric.JudgeRubric
}

// NewFileRubricStore constructs a FileRubricStore reading templates
// from dir on demand.
func NewFileRubricStore(dir string) *FileRubricStore {
	return &FileRubricStore{dir: dir, cache: make(map[string]rubric.JudgeRubric)}
}

func (s *FileRubricStore) Get(taskType string, version int) (rubric.JudgeRubric, error) {
	key := rubricKey(taskType, version)

	s.mu.RLock()
	if r, ok := s.cache[key]; ok {
		s.mu.RUnlock()
		return r, nil
	}
	s.mu.RUnlock()

	path := filepath.Join(s.dir, fmt.Sprintf("%s.v%d.json", taskType, version))
	f, err := os.Open(path)
	if err != nil {
		return rubric.JudgeRubric{}, fmt.Errorf("rubric store: open %s: %w", path, err)
	}
	defer f.Close()

	r, err := rubric.Load(f)
	if err != nil {
		return rubric.JudgeRubric{}, fmt.Errorf("rubric store: load %s: %w", path, err)
	}

	s.mu.Lock()
	s.cache[key] = r
	s.mu.Unlock()
	return r, nil
}

func rubricKey(taskType string, version int) string {
	return fmt.Sprintf("%s@%d", taskType, version)
}
