package sync

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"personal-kb/internal/nas"
)

func TestScheduler_StartStop(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes:     []nas.Note{},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	scheduler := NewScheduler(svc, 1*time.Second)
	scheduler.Start()

	// Give the scheduler time to run at least one sync
	time.Sleep(100 * time.Millisecond)

	scheduler.Stop()

	// Give the goroutine time to finish
	time.Sleep(50 * time.Millisecond)
}

func TestScheduler_DefaultInterval(t *testing.T) {
	if DefaultInterval != 30*time.Minute {
		t.Errorf("DefaultInterval = %v, want 30 minutes", DefaultInterval)
	}
}

func TestScheduler_RunSyncSuccess(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes: []nas.Note{
			{ID: "n1", Title: "Test", ModifiedTime: 1000},
		},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	scheduler := NewScheduler(svc, 10*time.Minute)

	// Call runSync directly to test it
	scheduler.runSync()

	// Note should have been synced
	if len(st.notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(st.notes))
	}
	if _, ok := st.notes["n1"]; !ok {
		t.Error("note n1 should exist")
	}
}

func TestScheduler_RunSyncFailure(t *testing.T) {
	nasClient := &mockNASClient{
		listErr: fmt.Errorf("connection failed"),
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	scheduler := NewScheduler(svc, 10*time.Minute)

	// Should not panic on error
	scheduler.runSync()

	// Store should remain empty
	if len(st.notes) != 0 {
		t.Errorf("expected 0 notes after failed sync, got %d", len(st.notes))
	}
}

func TestScheduler_MultipleStartStop(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes:     []nas.Note{},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	for i := 0; i < 3; i++ {
		scheduler := NewScheduler(svc, 1*time.Second)
		scheduler.Start()
		time.Sleep(50 * time.Millisecond)
		scheduler.Stop()
		time.Sleep(50 * time.Millisecond)
	}
}

func TestScheduler_ConcurrentStop(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes:     []nas.Note{},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	scheduler := NewScheduler(svc, 1*time.Second)
	scheduler.Start()
	time.Sleep(50 * time.Millisecond)

	// Stop from multiple goroutines should not panic
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduler.Stop()
		}()
	}
	wg.Wait()
}

func TestNewScheduler(t *testing.T) {
	nasClient := &mockNASClient{}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	interval := 5 * time.Minute
	scheduler := NewScheduler(svc, interval)

	if scheduler.service != svc {
		t.Error("service not set correctly")
	}
	if scheduler.interval != interval {
		t.Errorf("interval = %v, want %v", scheduler.interval, interval)
	}
	if scheduler.stopChan == nil {
		t.Error("stopChan should not be nil")
	}
}

func TestScheduler_String(t *testing.T) {
	nasClient := &mockNASClient{}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)

	scheduler := NewScheduler(svc, 5*time.Minute)
	expected := "Scheduler(interval=5m0s)"
	if scheduler.String() != expected {
		t.Errorf("String() = %q, want %q", scheduler.String(), expected)
	}
}
