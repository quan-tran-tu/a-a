package test

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

func Sleep(ctx context.Context, durationMs int) error {
	if durationMs < 0 {
		durationMs = 0
	}
	timer := time.NewTimer(time.Duration(durationMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func Fail(ctx context.Context, message string, durationMs int) error {
	if durationMs > 0 {
		timer := time.NewTimer(time.Duration(durationMs) * time.Millisecond)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	if message == "" {
		message = "test.Fail triggered"
	}

	return errors.New(message)
}

func SleepWithReturn(ctx context.Context, durationMs int) (map[string]any, error) {
	if durationMs < 0 {
		durationMs = 0
	}
	timer := time.NewTimer(time.Duration(durationMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return map[string]any{
			"status": "cancelled",
			"result": "none",
		}, ctx.Err()
	case <-timer.C:
		return map[string]any{
			"status": "ok",
			"result": uuid.NewString(),
		}, nil
	}
}
