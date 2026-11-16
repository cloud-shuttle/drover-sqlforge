package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/plan"
)

// PendingPlan stores an approved execution plan until apply_change consumes it.
type PendingPlan struct {
	Plan      *plan.ExecutionPlan
	CreatedAt time.Time
	// ModelName and ProposedSQL record the agent proposal the plan was built
	// from, so apply_change can re-validate it before executing.
	ModelName   string
	ProposedSQL string
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
	return s.PutProposal(p, "", "")
}

// PutProposal stores a plan together with the model SQL proposal it came from.
func (s *PlanStore) PutProposal(p *plan.ExecutionPlan, modelName, proposedSQL string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()

	id, err := newPlanID()
	if err != nil {
		return "", err
	}
	s.plans[id] = &PendingPlan{Plan: p, CreatedAt: time.Now(), ModelName: modelName, ProposedSQL: proposedSQL}
	return id, nil
}

// GetPending returns the stored plan and its proposal metadata.
func (s *PlanStore) GetPending(id string) (*PendingPlan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()

	pending, ok := s.plans[id]
	return pending, ok
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
