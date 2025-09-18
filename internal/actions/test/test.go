package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"a-a/internal/utils"
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

func HandleTestAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "sleep":
		sleepSecond, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			return nil, err
		}
		return nil, Sleep(ctx, sleepSecond)
	case "fail":
		afterSeconds, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			afterSeconds = 0
		}
		return nil, Fail(ctx, "", afterSeconds)
	case "sleep_with_return":
		sleepSecond, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			return nil, err
		}
		return SleepWithReturn(ctx, sleepSecond)
	default:
		return nil, fmt.Errorf("unknown test operation: %s", operation)
	}
}
