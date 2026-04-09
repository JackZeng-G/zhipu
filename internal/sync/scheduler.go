package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultInterval is the default interval between incremental syncs.
const DefaultInterval = 30 * time.Minute

// Scheduler runs periodic incremental syncs in the background.
type Scheduler struct {
	service  *SyncService
	interval time.Duration
	stopChan chan struct{}
	stopped  bool
	mu       sync.Mutex
}

// NewScheduler creates a new scheduler that calls IncrementalSync on the given interval.
func NewScheduler(service *SyncService, interval time.Duration) *Scheduler {
	return &Scheduler{
		service:  service,
		interval: interval,
		stopChan: make(chan struct{}),
		stopped:  false,
	}
}

// Start begins the background sync loop.
func (s *Scheduler) Start() {
	go func() {
		log.Printf("[sync] scheduler started with interval %s", s.interval)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Run an initial sync immediately
		s.runSync()

		for {
			select {
			case <-ticker.C:
				s.runSync()
			case <-s.stopChan:
				log.Printf("[sync] scheduler stopped")
				return
			}
		}
	}()
}

// Stop signals the background goroutine to stop.
// Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		close(s.stopChan)
		s.stopped = true
	}
}

// runSync executes a single incremental sync and logs any errors.
func (s *Scheduler) runSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := s.service.IncrementalSync(ctx); err != nil {
		log.Printf("[sync] incremental sync failed: %v", err)
		return
	}
	log.Printf("[sync] incremental sync completed successfully")
}

// String returns a human-readable description of the scheduler.
func (s *Scheduler) String() string {
	return fmt.Sprintf("Scheduler(interval=%s)", s.interval)
}
