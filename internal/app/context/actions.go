package context

import (
	"context"
	"fmt"
)

// Action represents a staged write operation.
type Action interface {
	// Execute performs the action.
	Execute(ctx context.Context) error

	// Rollback undoes the action if possible.
	Rollback(ctx context.Context) error

	// Description returns a human-readable description for logging.
	Description() string
}

// AddAction stages an action for later execution.
func (rc *RequestContext) AddAction(action Action) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.committed {
		return ErrAlreadyCommitted
	}

	rc.actions = append(rc.actions, action)
	return nil
}

// Commit executes all staged actions in order.
// On failure, rolls back executed actions in reverse order.
func (rc *RequestContext) Commit(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.committed {
		return ErrAlreadyCommitted
	}

	var executed []Action
	for _, action := range rc.actions {
		if err := action.Execute(ctx); err != nil {
			// Rollback in reverse order
			for i := len(executed) - 1; i >= 0; i-- {
				// Log rollback error but continue
				_ = executed[i].Rollback(ctx)
			}
			return fmt.Errorf("action %q failed: %w", action.Description(), err)
		}
		executed = append(executed, action)
	}

	rc.committed = true
	return nil
}

// Actions returns a copy of staged actions (for inspection/testing).
func (rc *RequestContext) Actions() []Action {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	result := make([]Action, len(rc.actions))
	copy(result, rc.actions)
	return result
}
