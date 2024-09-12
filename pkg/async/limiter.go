package async

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiter struct {
	rate     int64
	chBucket chan struct{}
	chClose  chan struct{}

	starter sync.Once
	running atomic.Bool
}

func NewRateLimiter(rate uint8) *RateLimiter {
	return &RateLimiter{
		rate:     int64(rate),
		chBucket: make(chan struct{}, 1),
		chClose:  make(chan struct{}, 1),
	}
}

func (l *RateLimiter) Start(ctx context.Context) error {
	l.starter.Do(func() {
		l.running.Store(true)
		go l.run(ctx)
	})

	return nil
}

func (l *RateLimiter) Call() error {
	if !l.running.Load() {
		return errors.New("rate limiter closed")
	}

	select {
	case <-l.chClose:
		return errors.New("rate limiter closed")
	case <-l.chBucket:
		return nil
	}
}

func (l *RateLimiter) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second / time.Duration(l.rate))

	for {
		select {
		case <-ctx.Done():
			l.running.Store(false)
			l.chClose <- struct{}{}

			close(l.chBucket)

			return
		case <-ticker.C:
			select {
			case l.chBucket <- struct{}{}:
			default:
			}
		}
	}
}
