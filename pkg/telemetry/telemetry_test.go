package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestEventCreation(t *testing.T) {
	evClick := NewButtonClickEvent(1001, 2001, "buy_item", "shop", 450*time.Millisecond)
	if evClick.Type != EventButtonClick || evClick.Action != "buy_item" || evClick.UserID != 1001 {
		t.Errorf("unexpected button click event: %+v", evClick)
	}

	evCmd := NewCommandEvent(1001, 2001, "/start", 12*time.Millisecond, nil)
	if evCmd.Type != EventCommand || evCmd.Action != "/start" || evCmd.Error != "" {
		t.Errorf("unexpected command event: %+v", evCmd)
	}

	evErr := NewErrorEvent(1001, 2001, "checkout", errors.New("insufficient funds"))
	if evErr.Type != EventError || evErr.Error != "insufficient funds" {
		t.Errorf("unexpected error event: %+v", evErr)
	}

	evCust := NewCustomEvent(1001, 2001, "level_up", map[string]any{"level": 5})
	if evCust.Type != EventCustom || evCust.Properties["level"] != 5 {
		t.Errorf("unexpected custom event: %+v", evCust)
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	rb := NewRingBuffer(3)

	ev1 := NewButtonClickEvent(1, 1, "btn_1", "", 0)
	ev2 := NewButtonClickEvent(2, 2, "btn_2", "", 0)
	ev3 := NewButtonClickEvent(3, 3, "btn_3", "", 0)
	ev4 := NewButtonClickEvent(4, 4, "btn_4", "", 0) // Should overwrite ev1

	rb.Add(ev1)
	rb.Add(ev2)
	rb.Add(ev3)
	if rb.Len() != 3 {
		t.Fatalf("expected len 3, got %d", rb.Len())
	}

	rb.Add(ev4)
	if rb.Len() != 3 {
		t.Fatalf("expected len 3 after wrap-around, got %d", rb.Len())
	}

	snapshot := rb.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected snapshot len 3, got %d", len(snapshot))
	}

	// Should be [ev2, ev3, ev4]
	if snapshot[0].Action != "btn_2" || snapshot[1].Action != "btn_3" || snapshot[2].Action != "btn_4" {
		t.Errorf("unexpected snapshot order: %s, %s, %s",
			snapshot[0].Action, snapshot[1].Action, snapshot[2].Action)
	}

	last2 := rb.Last(2)
	if len(last2) != 2 || last2[0].Action != "btn_4" || last2[1].Action != "btn_3" {
		t.Errorf("unexpected Last(2) order: %+v", last2)
	}

	filtered := rb.Filter(func(e *Event) bool { return e.UserID == 3 })
	if len(filtered) != 1 || filtered[0].UserID != 3 {
		t.Errorf("filter failed, got %+v", filtered)
	}
}

func TestRingBufferConcurrency(t *testing.T) {
	rb := NewRingBuffer(50)
	var wg sync.WaitGroup

	// Concurrently write 200 events across 10 goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				rb.Add(NewButtonClickEvent(int64(routineID), int64(j), "click", "", 0))
				_ = rb.Snapshot()
				_ = rb.Last(5)
			}
		}(i)
	}

	wg.Wait()
	if rb.Len() != 50 {
		t.Errorf("expected 50 items after concurrent fill, got %d", rb.Len())
	}
}

func TestJSONLogSinkAndCollector(t *testing.T) {
	var buf bytes.Buffer
	sink := NewJSONLogSink(&buf)

	collector := NewCollector(
		WithBufferSize(10),
		WithSink(sink),
	)
	defer collector.Close()

	ctx := context.Background()
	ev := NewButtonClickEvent(42, 100, "submit_form", "checkout", 120*time.Millisecond)
	collector.Record(ctx, ev)

	if len(collector.Events()) != 1 {
		t.Fatalf("expected 1 event in collector, got %d", len(collector.Events()))
	}

	// Verify JSON output
	var decoded Event
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON sink output: %v", err)
	}
	if decoded.Action != "submit_form" || decoded.UserID != 42 {
		t.Errorf("decoded event mismatch: %+v", decoded)
	}
}
