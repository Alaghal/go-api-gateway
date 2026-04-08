package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("closed state initially", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second)
		assert.True(t, cb.Allow())
		assert.Equal(t, StateClosed, cb.state)
	})

	t.Run("transitions to open after failures", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 1*time.Second)

		cb.Failure()
		assert.True(t, cb.Allow())
		assert.Equal(t, StateClosed, cb.state)

		cb.Failure()
		assert.False(t, cb.Allow())
		assert.Equal(t, StateOpen, cb.state)
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 100*time.Millisecond)
		cb.Failure()
		assert.False(t, cb.Allow())

		time.Sleep(150 * time.Millisecond)
		assert.True(t, cb.Allow())
		assert.Equal(t, StateHalfOpen, cb.state)
	})

	t.Run("closes after success in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 100*time.Millisecond)
		cb.Failure()
		time.Sleep(150 * time.Millisecond)
		assert.True(t, cb.Allow()) // Half-open

		cb.Success()
		assert.True(t, cb.Allow())
		assert.Equal(t, StateClosed, cb.state)
		assert.Equal(t, 0, cb.failures)
	})
}
