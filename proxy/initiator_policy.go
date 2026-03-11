package proxy

import "sync"

// InitiatorPolicy decides when requests should be marked as user-initiated.
type InitiatorPolicy struct {
	mu        sync.Mutex
	userEvery int
	count     int
}

func NewInitiatorPolicy(userEvery int) *InitiatorPolicy {
	if userEvery < 1 {
		userEvery = 1
	}
	return &InitiatorPolicy{userEvery: userEvery}
}

func (p *InitiatorPolicy) NextInitiator() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count++
	if p.count%(p.userEvery+1) == 0 {
		return "user"
	}
	return "agent"
}

func (p *InitiatorPolicy) GetUserEvery() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userEvery
}

func (p *InitiatorPolicy) SetUserEvery(userEvery int) {
	if userEvery < 1 {
		userEvery = 1
	}

	p.mu.Lock()
	p.userEvery = userEvery
	p.mu.Unlock()
}
