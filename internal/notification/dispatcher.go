package notification

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type Sender interface {
	Send(context.Context, Delivery, Event) error
}

type DispatchResult struct {
	Delivery Delivery
	Sent     bool
	Error    error
}

type Dispatcher struct {
	Senders       map[Channel]Sender
	Concurrency   int
	ContextPolicy DispatchContextPolicy
}

func (dispatcher Dispatcher) Dispatch(ctx context.Context, event Event, deliveries []Delivery) ([]DispatchResult, error) {
	if len(deliveries) == 0 {
		return []DispatchResult{}, nil
	}
	concurrency := dispatcher.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > 32 {
		concurrency = 32
	}
	dispatchCtx, cancelDispatch, _, err := dispatchContext(ctx, dispatcher.ContextPolicy)
	if err != nil {
		return nil, err
	}
	defer cancelDispatch()
	results := make([]DispatchResult, len(deliveries))
	jobs := make(chan int)
	var group sync.WaitGroup
	for workerIndex := 0; workerIndex < concurrency; workerIndex++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				result := DispatchResult{Delivery: deliveries[index]}
				// Re-check before sending: if the dispatch was cancelled while
				// this job was queued, never start the send so it cannot hold a
				// connection. Sends already in flight return promptly because
				// they share the cancelled dispatch context.
				if err := dispatchCtx.Err(); err != nil {
					result.Error = err
					results[index] = result
					continue
				}
				sender := dispatcher.Senders[deliveries[index].Channel]
				if sender == nil {
					result.Error = fmt.Errorf("no sender for channel %s", deliveries[index].Channel)
				} else if err := sender.Send(dispatchCtx, deliveries[index], event); err != nil {
					result.Error = err
				} else {
					result.Sent = true
				}
				results[index] = result
			}
		}()
	}
	// Feed deliveries in submission order, but stop the moment the dispatch
	// context is cancelled so notifications that have not been scheduled yet
	// never start. `fed` tracks how many were actually handed to a worker.
	fed := 0
feed:
	for index := range deliveries {
		select {
		case jobs <- index:
			fed = index + 1
		case <-dispatchCtx.Done():
			break feed
		}
	}
	close(jobs)
	group.Wait()
	if err := dispatchCtx.Err(); err != nil {
		// Fill every delivery that was never scheduled, preserving the original
		// delivery order in the returned results.
		for remaining := fed; remaining < len(deliveries); remaining++ {
			results[remaining] = DispatchResult{Delivery: deliveries[remaining], Error: err}
		}
		// If cancellation arrived after every delivery had already been sent,
		// nothing was interrupted: report success so normal notifications are
		// unaffected by a late cancellation.
		for _, result := range results {
			if !result.Sent {
				return results, err
			}
		}
		return results, nil
	}
	return results, nil
}

func Failed(results []DispatchResult) []DispatchResult {
	failed := make([]DispatchResult, 0)
	for _, result := range results {
		if !result.Sent || result.Error != nil {
			failed = append(failed, result)
		}
	}
	sort.Slice(failed, func(i, j int) bool { return failed[i].Delivery.ID < failed[j].Delivery.ID })
	return failed
}

func JoinErrors(results []DispatchResult) error {
	errorsToJoin := make([]error, 0)
	for _, result := range results {
		if result.Error != nil {
			errorsToJoin = append(errorsToJoin, fmt.Errorf("delivery %s: %w", result.Delivery.ID, result.Error))
		}
	}
	return errors.Join(errorsToJoin...)
}
