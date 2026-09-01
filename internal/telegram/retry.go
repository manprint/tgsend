package telegram

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/message"
)

const (
	maxRetries        = 2
	maxCumulativeWait = 60 * time.Second
)

// Sleeper abstracts waiting between attempts so retry behavior can be tested
// without making tests wait in real time.
type Sleeper interface {
	Sleep(context.Context, time.Duration) error
}

// NewProductionSleeper returns the context-aware timer used by native sends.
func NewProductionSleeper() Sleeper {
	return timerSleeper{}
}

type timerSleeper struct{}

func (timerSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (client *Client) sendWithRetry(ctx context.Context, chatID string, chunk message.Chunk) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var cumulative time.Duration
	for retry := 0; ; retry++ {
		messageID, err := client.sendAttempt(ctx, chatID, chunk)
		if err == nil {
			return messageID, nil
		}
		failure, ok := retryFailure(err)
		if !ok || (failure.statusCode != 429 && failure.errorCode != 429) {
			return 0, err
		}
		delay, ok := retryDelay(failure.retryAfter)
		if !ok || retry >= maxRetries || delay > maxCumulativeWait-cumulative {
			return 0, rateLimitExhausted(failure)
		}
		if err := client.sleeper.Sleep(ctx, delay); err != nil {
			return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram retry wait was interrupted", err)
		}
		cumulative += delay
	}
}

func retryFailure(err error) (*attemptFailure, bool) {
	var failure *attemptFailure
	return failure, errors.As(err, &failure) && failure != nil
}

func retryDelay(seconds int) (time.Duration, bool) {
	if seconds <= 0 || int64(seconds) > int64(math.MaxInt64/time.Second) {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func rateLimitExhausted(failure *attemptFailure) error {
	return apperr.New(apperr.KindRateLimit, apperr.CodeTelegramRateLimited, "Telegram rate limit retry policy was exhausted", failure)
}
