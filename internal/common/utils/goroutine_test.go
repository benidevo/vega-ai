package utils

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestSafeGo(t *testing.T) {
	t.Run("executes_function_successfully", func(t *testing.T) {
		executed := make(chan bool, 1)

		SafeGo(func() {
			executed <- true
		})

		select {
		case <-executed:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Function was not executed")
		}
	})

	t.Run("recovers_from_panic", func(t *testing.T) {
		var buf bytes.Buffer
		oldLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		defer func() { log.Logger = oldLogger }()

		done := make(chan bool, 1)

		SafeGo(func() {
			defer func() { done <- true }()
			panic("test panic")
		})

		select {
		case <-done:
			time.Sleep(10 * time.Millisecond)
			logOutput := buf.String()
			assert.Contains(t, logOutput, "Panic recovered in goroutine")
			assert.Contains(t, logOutput, "test panic")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Function did not complete")
		}
	})

	t.Run("handles_nil_function", func(t *testing.T) {
		var buf bytes.Buffer
		oldLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		defer func() { log.Logger = oldLogger }()

		SafeGo(nil)
		time.Sleep(10 * time.Millisecond)
		logOutput := buf.String()
		assert.Contains(t, logOutput, "SafeGo called with nil function")
	})

	t.Run("concurrent_execution", func(t *testing.T) {
		var wg sync.WaitGroup
		counter := 0
		var mu sync.Mutex

		for i := 0; i < 10; i++ {
			wg.Add(1)
			SafeGo(func() {
				defer wg.Done()
				mu.Lock()
				counter++
				mu.Unlock()
			})
		}

		wg.Wait()
		assert.Equal(t, 10, counter)
	})
}

func TestSafeGoWithName(t *testing.T) {
	t.Run("executes_function_with_name", func(t *testing.T) {
		executed := make(chan bool, 1)

		SafeGoWithName("test-goroutine", func() {
			executed <- true
		})

		select {
		case <-executed:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Function was not executed")
		}
	})

	t.Run("logs_name_on_panic", func(t *testing.T) {
		var buf bytes.Buffer
		oldLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		defer func() { log.Logger = oldLogger }()

		done := make(chan bool, 1)

		SafeGoWithName("named-routine", func() {
			defer func() { done <- true }()
			panic("named panic")
		})

		select {
		case <-done:
			time.Sleep(10 * time.Millisecond)
			logOutput := buf.String()
			assert.Contains(t, logOutput, "named-routine")
			assert.Contains(t, logOutput, "named panic")
			assert.Contains(t, logOutput, "Panic recovered in goroutine")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Function did not complete")
		}
	})

	t.Run("handles_nil_function_with_name", func(t *testing.T) {
		var buf bytes.Buffer
		oldLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		defer func() { log.Logger = oldLogger }()

		SafeGoWithName("nil-test", nil)

		time.Sleep(10 * time.Millisecond)
		logOutput := buf.String()
		assert.Contains(t, logOutput, "SafeGoWithName called with nil function")
		assert.Contains(t, logOutput, "nil-test")
	})

	t.Run("multiple_named_goroutines", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(map[string]bool)
		var mu sync.Mutex

		names := []string{"worker-1", "worker-2", "worker-3"}

		for _, name := range names {
			wg.Add(1)
			localName := name
			SafeGoWithName(localName, func() {
				defer wg.Done()
				mu.Lock()
				results[localName] = true
				mu.Unlock()
			})
		}

		wg.Wait()
		assert.Len(t, results, 3)
		for _, name := range names {
			assert.True(t, results[name])
		}
	})
}

func TestPanicRecoveryStackTrace(t *testing.T) {
	t.Run("includes_stack_trace_in_log", func(t *testing.T) {
		var buf bytes.Buffer
		oldLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		defer func() { log.Logger = oldLogger }()

		done := make(chan bool, 1)

		SafeGo(func() {
			defer func() { done <- true }()
			causePanic()
		})

		select {
		case <-done:
			time.Sleep(10 * time.Millisecond)
			logOutput := buf.String()
			assert.True(t, strings.Contains(logOutput, "goroutine") || strings.Contains(logOutput, "stack"))
			assert.Contains(t, logOutput, "causePanic")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Function did not complete")
		}
	})
}

func causePanic() {
	panic("intentional panic for testing")
}
