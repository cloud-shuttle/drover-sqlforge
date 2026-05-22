package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/drover-org/drover-sqlforge/internal/plan"
)

// PendingPlan stores an approved execution plan until apply_change consumes it.
type PendingPlan struct {
	Plan      *plan.ExecutionPlan
	CreatedAt time.Time
}

// PlanStore holds ephemeral execution plans keyed by plan_id.
type PlanStore struct {
	mu    sync.Mutex
	plans map[string]*PendingPlan
	ttl   time.Duration
}

func NewPlanStore() *PlanStore {
	return &PlanStore{
		plans: make(map[string]*PendingPlan),
		ttl:   2 * time.Hour,
	}
}

func (s *PlanStore) Put(p *plan.ExecutionPlan) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()

	id, err := newPlanID()
	if err != nil {
		return "", err
	}
	s.plans[id] = &PendingPlan{Plan: p, CreatedAt: time.Now()}
	return id, nil
}

func (s *PlanStore) Get(id string) (*plan.ExecutionPlan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()

	pending, ok := s.plans[id]
	if !ok {
		return nil, false
	}
	return pending.Plan, true
}

func (s *PlanStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.plans, id)
}

func (s *PlanStore) evictLocked() {
	cutoff := time.Now().Add(-s.ttl)
	for id, p := range s.plans {
		if p.CreatedAt.Before(cutoff) {
			delete(s.plans, id)
		}
	}
}

func newPlanID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
