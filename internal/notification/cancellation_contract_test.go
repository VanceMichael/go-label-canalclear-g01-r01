package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type cancellationAwareSender struct{ started chan string }

func (sender *cancellationAwareSender) Send(ctx context.Context, delivery Delivery, _ Event) error {
	sender.started <- delivery.ID
	<-ctx.Done()
	return ctx.Err()
}

func TestCancelledDispatchStopsOutstandingPortNotifications(t *testing.T) {
	sender := &cancellationAwareSender{started: make(chan string, 2)}
	dispatcher := Dispatcher{
		Senders:       map[Channel]Sender{ChannelEmail: sender},
		Concurrency:   1,
		ContextPolicy: DefaultDispatchContextPolicy(),
	}
	deliveries := []Delivery{
		{ID: "clearance-email", Channel: ChannelEmail, Destination: "clearance@example.test"},
		{ID: "passage-email", Channel: ChannelEmail, Destination: "passage@example.test"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		results []DispatchResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := dispatcher.Dispatch(ctx, Event{ID: "port-operation-cancelled"}, deliveries)
		done <- outcome{results: results, err: err}
	}()
	select {
	case <-sender.started:
	case <-time.After(time.Second):
		t.Fatal("notification sending did not start")
	}
	cancel()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("Dispatch error=%v, want context cancellation", got.err)
		}
		if len(got.results) != len(deliveries) {
			t.Fatalf("result count=%d, want %d", len(got.results), len(deliveries))
		}
		for _, result := range got.results {
			if result.Sent || !errors.Is(result.Error, context.Canceled) {
				t.Fatalf("delivery %s result=%+v", result.Delivery.ID, result)
			}
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("dispatch did not return after caller cancellation")
	}
}
