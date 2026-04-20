package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"goshare/app/network"
	"goshare/internal/utils"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type Task struct {
	ID          string
	FilePath    string
	Target      string
	FileName    string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Transferred int64
	Total       int64
	LastError   string
}

type Manager struct {
	connection *network.ConnectionManager

	mu       sync.RWMutex
	tasks    map[string]*Task
	cancelFn map[string]context.CancelFunc
}

func NewManager(conn *network.ConnectionManager) *Manager {
	return &Manager{
		connection: conn,
		tasks:      make(map[string]*Task),
		cancelFn:   make(map[string]context.CancelFunc),
	}
}

func (m *Manager) StartTransfer(ctx context.Context, filePath, target string) (string, error) {
	id := utils.NewID()
	now := time.Now()
	task := &Task{
		ID:        id,
		FilePath:  filePath,
		Target:    target,
		FileName:  filepath.Base(filePath),
		Status:    StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	taskCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancelFn[id] = cancel
	task.Status = StatusRunning
	task.UpdatedAt = time.Now()
	m.mu.Unlock()

	go func() {
		err := m.connection.SendFile(taskCtx, target, filePath, func(done, total int64) {
			m.mu.Lock()
			defer m.mu.Unlock()
			t := m.tasks[id]
			if t == nil {
				return
			}
			t.Transferred = done
			t.Total = total
			t.UpdatedAt = time.Now()
		})

		m.mu.Lock()
		defer m.mu.Unlock()
		defer delete(m.cancelFn, id)

		t := m.tasks[id]
		if t == nil {
			return
		}

		t.UpdatedAt = time.Now()
		switch {
		case err == nil:
			t.Status = StatusCompleted
		case taskCtx.Err() != nil:
			t.Status = StatusCanceled
			t.LastError = taskCtx.Err().Error()
		default:
			t.Status = StatusFailed
			t.LastError = err.Error()
		}
	}()

	return id, nil
}

func (m *Manager) CancelTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancel := m.cancelFn[id]
	if cancel == nil {
		return fmt.Errorf("transfer %s not found or already finished", id)
	}
	cancel()
	return nil
}

func (m *Manager) ListTasks() []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, *t)
	}
	return result
}
