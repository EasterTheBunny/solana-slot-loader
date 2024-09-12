package async_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/easterthebunny/solana-slot-loader/pkg/async"
)

func TestLimiter(t *testing.T) {
	limiter := async.NewRateLimiter(2)

	ctx, cancel := context.WithCancel(context.Background())

	assert.NoError(t, limiter.Start(ctx))

	start := time.Now()
	assert.NoError(t, limiter.Call())

	time.Sleep(500 * time.Millisecond)

	assert.NoError(t, limiter.Call())
	assert.NoError(t, limiter.Call())
	assert.True(t, time.Since(start) < 1600*time.Millisecond)

	cancel()

	assert.NotNil(t, limiter.Call())
}
