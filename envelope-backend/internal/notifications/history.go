package notifications

import (
	"fmt"
	"sync"
	"time"
)

// Operation — запись в истории изменений конвертов, сделанных механизмом уведомлений.
// Поля повторяют структуру из прототипа; суммы, если есть, — в копейках.
type Operation struct {
	Date          time.Time `json:"date"`
	Type          string    `json:"type"`
	AmountKopecks int64     `json:"amount_kopecks,omitempty"`
	From          string    `json:"from,omitempty"`
	To            string    `json:"to,omitempty"`
	Description   string    `json:"description"`
}

// History хранит историю в JSON-файле. Файл (и каталог) создаются при открытии, если их нет.
type History struct {
	mu   sync.Mutex
	path string
	ops  []Operation
}

func OpenHistory(path string) (*History, error) {
	ops, err := loadJSONFile[[]Operation](path, []byte("[]\n"))
	if err != nil {
		return nil, fmt.Errorf("open history: %w", err)
	}
	return &History{path: path, ops: ops}, nil
}

// Record добавляет операцию и сразу сохраняет файл.
func (h *History) Record(op Operation) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if op.Date.IsZero() {
		op.Date = time.Now()
	}
	h.ops = append(h.ops, op)
	return writeJSONFile(h.path, h.ops)
}

// List возвращает копию истории в порядке добавления.
func (h *History) List() []Operation {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Operation(nil), h.ops...)
}
