package notification

import (
	"context"
	"fmt"
	"time"
)

type DispatchContextPolicy struct {
	MaximumDuration time.Duration
	PreserveValues  bool
}

type DispatchContextMetadata struct {
	StartedAt time.Time
	Deadline  time.Time
	Bounded   bool
}

func DefaultDispatchContextPolicy() DispatchContextPolicy {
	return DispatchContextPolicy{MaximumDuration: 30 * time.Second, PreserveValues: true}
}

func (policy DispatchContextPolicy) Validate() error {
	if policy.MaximumDuration <= 0 || policy.MaximumDuration > 5*time.Minute {
		return fmt.Errorf("invalid notification dispatch duration")
	}
	return nil
}

func (policy DispatchContextPolicy) Derive(parent context.Context, now time.Time) (context.Context, context.CancelFunc, DispatchContextMetadata, error) {
	if parent == nil {
		return nil, nil, DispatchContextMetadata{}, fmt.Errorf("notification dispatch context is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if err := policy.Validate(); err != nil {
		return nil, nil, DispatchContextMetadata{}, err
	}
	base := context.Background()
	if policy.PreserveValues {
		base = context.WithoutCancel(parent)
	}
	deadline := now.Add(policy.MaximumDuration)
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	derived, cancel := context.WithDeadline(base, deadline)
	metadata := DispatchContextMetadata{StartedAt: now, Deadline: deadline.UTC(), Bounded: true}
	return derived, cancel, metadata, nil
}

func dispatchContext(parent context.Context, policy DispatchContextPolicy) (context.Context, context.CancelFunc, DispatchContextMetadata, error) {
	if policy.MaximumDuration == 0 {
		policy = DefaultDispatchContextPolicy()
	}
	return policy.Derive(parent, time.Now().UTC())
}
