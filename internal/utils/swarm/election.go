package swarm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoLeader           = errors.New("no leader elected")
	ErrElectionInProgress = errors.New("election already in progress")
)

type LeaderElection struct {
	config        *SwarmConfig
	registry      *BackendRegistry
	events        *EventBus
	currentLeader atomic.Pointer[Backend]
	inElection    atomic.Bool
	leaseMu       sync.Mutex
	leaseExpiry   time.Time
	leaseHolder   BackendID
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewLeaderElection(config *SwarmConfig, registry *BackendRegistry, events *EventBus) *LeaderElection {
	if config == nil {
		config = DefaultSwarmConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderElection{
		config:   config,
		registry: registry,
		events:   events,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (le *LeaderElection) Start() {
	if !le.config.LeaderElectionEnabled {
		return
	}
	le.wg.Add(1)
	go le.electionLoop()
}

func (le *LeaderElection) electionLoop() {
	defer le.wg.Done()
	ticker := time.NewTicker(le.config.LeaseDuration / 2)
	defer ticker.Stop()

	for {
		select {
		case <-le.ctx.Done():
			return
		case <-ticker.C:
			le.tryElect()
		}
	}
}

func (le *LeaderElection) tryElect() {
	if !le.inElection.CompareAndSwap(false, true) {
		return
	}
	defer le.inElection.Store(false)

	backends := le.registry.GetHealthy()
	if len(backends) == 0 {
		le.stepDown()
		return
	}

	var candidate *Backend
	for _, b := range backends {
		if candidate == nil || b.RegisteredAt.Before(candidate.RegisteredAt) {
			candidate = b
		}
	}

	if candidate == nil {
		le.stepDown()
		return
	}

	le.leaseMu.Lock()
	now := time.Now()
	if now.Before(le.leaseExpiry) && le.leaseHolder == candidate.ID {
		le.leaseExpiry = now.Add(le.config.LeaseDuration)
		le.leaseMu.Unlock()
		return
	}
	le.leaseMu.Unlock()

	le.leaseMu.Lock()
	le.leaseHolder = candidate.ID
	le.leaseExpiry = now.Add(le.config.LeaseDuration)
	le.leaseMu.Unlock()

	oldLeader := le.currentLeader.Swap(candidate)
	if oldLeader != candidate {
		if oldLeader != nil {
			le.events.Publish(SwarmEvent{
				Type:      EventLeaderLost,
				Timestamp: time.Now(),
				BackendID: oldLeader.ID,
			})
		}
		le.events.Publish(SwarmEvent{
			Type:      EventLeaderElected,
			Timestamp: time.Now(),
			BackendID: candidate.ID,
			Data: map[string]interface{}{
				"name": candidate.Name,
			},
		})
	}
}

func (le *LeaderElection) stepDown() {
	oldLeader := le.currentLeader.Load()
	if oldLeader != nil {
		le.currentLeader.Store(nil)
		le.events.Publish(SwarmEvent{
			Type:      EventLeaderLost,
			Timestamp: time.Now(),
			BackendID: oldLeader.ID,
		})
	}
	le.leaseMu.Lock()
	le.leaseHolder = ""
	le.leaseExpiry = time.Time{}
	le.leaseMu.Unlock()
}

func (le *LeaderElection) GetLeader() (*Backend, error) {
	leader := le.currentLeader.Load()
	if leader == nil {
		return nil, ErrNoLeader
	}
	return leader, nil
}

func (le *LeaderElection) IsLeader(backendID BackendID) bool {
	leader := le.currentLeader.Load()
	return leader != nil && leader.ID == backendID
}

func (le *LeaderElection) GetLeaseInfo() (BackendID, time.Time) {
	le.leaseMu.Lock()
	defer le.leaseMu.Unlock()
	return le.leaseHolder, le.leaseExpiry
}

func (le *LeaderElection) RenewLease(backendID BackendID) error {
	le.leaseMu.Lock()
	defer le.leaseMu.Unlock()
	if le.leaseHolder != backendID {
		return errors.New("not lease holder")
	}
	if time.Now().After(le.leaseExpiry) {
		return ErrLeaseExpired
	}
	le.leaseExpiry = time.Now().Add(le.config.LeaseDuration)
	return nil
}

func (le *LeaderElection) ForceElection() {
	le.inElection.Store(false)
	le.tryElect()
}

func (le *LeaderElection) Stop() {
	le.cancel()
	le.wg.Wait()
	le.stepDown()
}
