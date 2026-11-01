package notifications

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"
)

const resolvedRetention = 14 * 24 * time.Hour

// state — всё, что механизм уведомлений хранит между запусками.
type state struct {
	NextID             int64          `json:"next_id"`
	Initialized        bool           `json:"initialized"`
	LastTransactionID  int64          `json:"last_transaction_id"`
	SurveyCompletedAt  *time.Time     `json:"survey_completed_at,omitempty"`
	SurveySnoozedUntil *time.Time     `json:"survey_snoozed_until,omitempty"`
	Priorities         map[int64]int  `json:"priorities,omitempty"`
	EmptyNotified      map[int64]bool `json:"empty_notified,omitempty"`
	Notifications      []Notification `json:"notifications"`
}

func (st *state) find(id int64) *Notification {
	for i := range st.Notifications {
		if st.Notifications[i].ID == id {
			return &st.Notifications[i]
		}
	}
	return nil
}

func (st *state) hasOpen(kind Kind) bool {
	for _, n := range st.Notifications {
		if n.Kind == kind && !n.Resolved {
			return true
		}
	}
	return false
}

// add создаёт уведомление. Возвращает false, если уже есть открытое с таким же ключом.
func (st *state) add(n Notification, now time.Time) bool {
	if n.Key != "" {
		for _, existing := range st.Notifications {
			if existing.Key == n.Key && !existing.Resolved {
				return false
			}
		}
	}
	n.ID = st.NextID
	st.NextID++
	n.CreatedAt = now
	if n.Actions == nil {
		n.Actions = []Action{}
	}
	st.Notifications = append(st.Notifications, n)
	return true
}

// prune удаляет давно закрытые уведомления, чтобы файл не рос бесконечно.
func (st *state) prune(now time.Time) bool {
	kept := st.Notifications[:0]
	for _, n := range st.Notifications {
		if n.Resolved && now.Sub(n.CreatedAt) > resolvedRetention {
			continue
		}
		kept = append(kept, n)
	}
	changed := len(kept) != len(st.Notifications)
	st.Notifications = kept
	return changed
}

// store — состояние в памяти с сохранением в JSON-файл (создаётся, если его нет).
type store struct {
	mu   sync.Mutex
	path string
	s    state
}

func openStore(path string) (*store, error) {
	initial := []byte("{\n  \"next_id\": 1,\n  \"notifications\": []\n}\n")
	loaded, err := loadJSONFile[state](path, initial)
	if err != nil {
		return nil, fmt.Errorf("open notifications state: %w", err)
	}
	if loaded.NextID < 1 {
		loaded.NextID = 1
	}
	if loaded.Notifications == nil {
		loaded.Notifications = []Notification{}
	}
	return &store{path: filepath.Clean(path), s: loaded}, nil
}

// update выполняет fn под блокировкой и сохраняет файл, если fn сообщила об изменениях.
func (st *store) update(fn func(s *state) (bool, error)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	changed, err := fn(&st.s)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := writeJSONFile(st.path, st.s); err != nil {
		log.Printf("notifications: save state: %v", err)
		return fmt.Errorf("save notifications state: %w", err)
	}
	return nil
}

// view читает состояние под блокировкой, не изменяя его.
func (st *store) view(fn func(s *state)) {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.s)
}

func (st *store) get(id int64) (Notification, error) {
	var out Notification
	found := false
	st.view(func(s *state) {
		if n := s.find(id); n != nil {
			out = *n
			found = true
		}
	})
	if !found {
		return Notification{}, ErrNotificationNotFound
	}
	return out, nil
}

// resolve помечает уведомление закрытым и прочитанным; extra позволяет заодно поменять состояние.
func (st *store) resolve(id int64, extra func(s *state)) (Notification, error) {
	var out Notification
	err := st.update(func(s *state) (bool, error) {
		n := s.find(id)
		if n == nil {
			return false, ErrNotificationNotFound
		}
		n.Resolved = true
		n.Read = true
		if extra != nil {
			extra(s)
		}
		out = *n
		return true, nil
	})
	return out, err
}

func (st *store) priorities() map[int64]int {
	out := make(map[int64]int)
	st.view(func(s *state) {
		for id, p := range s.Priorities {
			out[id] = p
		}
	})
	return out
}
