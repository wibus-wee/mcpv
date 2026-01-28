package tasks

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"mcpd/internal/domain"
)

const (
	defaultPollInterval  = 5 * time.Second
	defaultListLimit     = 50
	defaultPurgeInterval = 1 * time.Minute
)

// Manager implements an in-memory task manager.
type Manager struct {
	mu        sync.Mutex
	tasks     map[string]*taskState
	order     []string
	now       func() time.Time
	stopPurge chan struct{}
	purgeDone chan struct{}
}

type taskState struct {
	task      domain.Task
	result    domain.TaskResult
	done      chan struct{}
	cancel    context.CancelFunc
	expiresAt *time.Time
	owner     string
}

// NewManager constructs a new task manager.
func NewManager() *Manager {
	m := &Manager{
		tasks:     make(map[string]*taskState),
		order:     make([]string, 0),
		now:       time.Now,
		stopPurge: make(chan struct{}),
		purgeDone: make(chan struct{}),
	}
	go m.backgroundPurge()
	return m
}

// Stop gracefully stops the background purge goroutine.
func (m *Manager) Stop() {
	close(m.stopPurge)
	<-m.purgeDone
}

// backgroundPurge periodically cleans up expired tasks.
func (m *Manager) backgroundPurge() {
	defer close(m.purgeDone)
	ticker := time.NewTicker(defaultPurgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopPurge:
			return
		case <-ticker.C:
			m.mu.Lock()
			m.purgeExpiredLocked()
			m.mu.Unlock()
		}
	}
}

// Create registers and starts a new task.
func (m *Manager) Create(ctx context.Context, owner string, opts domain.TaskCreateOptions, run domain.TaskRunner) (domain.Task, error) {
	if run == nil {
		return domain.Task{}, errors.New("task runner is required")
	}
	now := m.now()
	taskID := newTaskID(now)

	pollInterval := opts.PollInterval
	if pollInterval == nil {
		ms := int64(defaultPollInterval / time.Millisecond)
		pollInterval = &ms
	}

	task := domain.Task{
		TaskID:        taskID,
		Status:        domain.TaskStatusWorking,
		StatusMessage: "The operation is now in progress.",
		CreatedAt:     now,
		LastUpdatedAt: now,
		TTL:           opts.TTL,
		PollInterval:  pollInterval,
	}

	var expiresAt *time.Time
	if opts.TTL != nil && *opts.TTL > 0 {
		exp := now.Add(time.Duration(*opts.TTL) * time.Millisecond)
		expiresAt = &exp
	}

	taskCtx, cancel := context.WithCancel(ctx)
	state := &taskState{
		task:      task,
		result:    domain.TaskResult{Status: domain.TaskStatusWorking},
		done:      make(chan struct{}),
		cancel:    cancel,
		expiresAt: expiresAt,
		owner:     owner,
	}

	m.mu.Lock()
	m.tasks[taskID] = state
	m.order = append(m.order, taskID)
	m.mu.Unlock()

	go m.runTask(taskCtx, taskID, run)

	return task, nil
}

// Get returns task metadata without blocking.
func (m *Manager) Get(_ context.Context, owner, taskID string) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked()
	state, ok := m.tasks[taskID]
	if !ok {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	if owner != "" && state.owner != owner {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	return state.task, nil
}

// List returns a paginated list of tasks.
func (m *Manager) List(ctx context.Context, owner, cursor string, limit int) (domain.TaskPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.TaskPage{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked()

	if limit <= 0 {
		limit = defaultListLimit
	}

	start := 0
	if cursor != "" {
		val, err := strconv.Atoi(cursor)
		if err != nil || val < 0 {
			return domain.TaskPage{}, domain.ErrInvalidCursor
		}
		start = val
	}

	if start >= len(m.order) {
		return domain.TaskPage{Tasks: []domain.Task{}}, nil
	}

	end := min(start+limit, len(m.order))

	tasks := make([]domain.Task, 0, end-start)
	for _, id := range m.order[start:end] {
		state, ok := m.tasks[id]
		if !ok {
			continue
		}
		if owner != "" && state.owner != owner {
			continue
		}
		tasks = append(tasks, state.task)
	}

	nextCursor := ""
	if end < len(m.order) {
		nextCursor = strconv.Itoa(end)
	}

	return domain.TaskPage{
		Tasks:      tasks,
		NextCursor: nextCursor,
	}, nil
}

// Result blocks until the task finishes or the context is cancelled.
func (m *Manager) Result(ctx context.Context, owner, taskID string) (domain.TaskResult, error) {
	state, ok := m.getState(owner, taskID)
	if !ok {
		return domain.TaskResult{}, domain.ErrTaskNotFound
	}
	select {
	case <-ctx.Done():
		return domain.TaskResult{}, ctx.Err()
	case <-state.done:
		return state.result, nil
	}
}

// Cancel cancels a running task.
func (m *Manager) Cancel(ctx context.Context, owner, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked()
	state, ok := m.tasks[taskID]
	if !ok {
		return domain.ErrTaskNotFound
	}
	if owner != "" && state.owner != owner {
		return domain.ErrTaskNotFound
	}
	if isTerminal(state.task.Status) {
		return fmt.Errorf("task already completed")
	}
	if state.cancel != nil {
		state.cancel()
	}
	now := m.now()
	state.task.Status = domain.TaskStatusCancelled
	state.task.StatusMessage = "The task was cancelled."
	state.task.LastUpdatedAt = now
	state.result = domain.TaskResult{Status: domain.TaskStatusCancelled}
	close(state.done)
	return nil
}

func (m *Manager) runTask(ctx context.Context, taskID string, run domain.TaskRunner) {
	runResult, err := run(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.tasks[taskID]
	if !ok {
		return
	}
	if isTerminal(state.task.Status) {
		return
	}

	now := m.now()
	state.task.LastUpdatedAt = now
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		state.task.Status = domain.TaskStatusCancelled
		state.task.StatusMessage = "The task was cancelled."
		state.result = domain.TaskResult{Status: domain.TaskStatusCancelled}
	case err != nil:
		state.task.Status = domain.TaskStatusFailed
		state.task.StatusMessage = err.Error()
		state.result = domain.TaskResult{Status: domain.TaskStatusFailed}
	case runResult.ProtocolError != nil:
		state.task.Status = domain.TaskStatusFailed
		state.task.StatusMessage = runResult.ProtocolError.Message
		state.result = domain.TaskResult{Status: domain.TaskStatusFailed, Error: runResult.ProtocolError}
	default:
		state.task.Status = domain.TaskStatusCompleted
		state.task.StatusMessage = "The task completed successfully."
		state.result = domain.TaskResult{Status: domain.TaskStatusCompleted, Result: runResult.Result}
	}
	close(state.done)
}

func (m *Manager) getState(owner, taskID string) (*taskState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked()
	state, ok := m.tasks[taskID]
	if !ok {
		return nil, false
	}
	if owner != "" && state.owner != owner {
		return nil, false
	}
	return state, ok
}

func (m *Manager) purgeExpiredLocked() {
	if len(m.tasks) == 0 {
		return
	}
	now := m.now()
	filtered := m.order[:0]
	for _, id := range m.order {
		state, ok := m.tasks[id]
		if !ok {
			continue
		}
		if state.expiresAt != nil && state.expiresAt.Before(now) {
			if state.cancel != nil {
				state.cancel()
			}
			delete(m.tasks, id)
			continue
		}
		filtered = append(filtered, id)
	}
	m.order = filtered
}

func newTaskID(now time.Time) string {
	return fmt.Sprintf("task-%d", now.UnixNano())
}

func isTerminal(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusCompleted, domain.TaskStatusFailed, domain.TaskStatusCancelled:
		return true
	case domain.TaskStatusWorking, domain.TaskStatusInputRequired:
		return false
	default:
		return false
	}
}
