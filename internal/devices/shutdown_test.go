package devices

import (
	"errors"
	"fmt"
	"testing"
)

func TestShutdownHook_RegisterAndShutdown(t *testing.T) {
	hook := NewShutdownHook()

	called := false
	hook.Register("test-hook", func() error {
		called = true
		return nil
	})

	if hook.Count() != 1 {
		t.Errorf("Expected 1 hook, got %d", hook.Count())
	}

	err := hook.Shutdown()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !called {
		t.Error("Hook was not called")
	}

	if hook.Count() != 0 {
		t.Errorf("Expected hooks to be cleared, got %d", hook.Count())
	}
}

func TestShutdownHook_ErrorHandling(t *testing.T) {
	hook := NewShutdownHook()

	hook.Register("success", func() error { return nil })
	hook.Register("failure", func() error { return errors.New("cleanup failed") })
	hook.Register("success2", func() error { return nil })

	err := hook.Shutdown()

	if err == nil {
		t.Error("Expected error from failed hook")
	}

	// all hooks should still be cleared
	if hook.Count() != 0 {
		t.Errorf("Expected hooks to be cleared even after error, got %d", hook.Count())
	}
}

func TestShutdownHook_EmptyShutdown(t *testing.T) {
	hook := NewShutdownHook()

	err := hook.Shutdown()
	if err != nil {
		t.Errorf("Empty shutdown should not error: %v", err)
	}
}

func TestShutdownHook_ConcurrentRegister(t *testing.T) {
	hook := NewShutdownHook()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			hook.Register(fmt.Sprintf("hook-%d", n), func() error { return nil })
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if hook.Count() != 10 {
		t.Errorf("Expected 10 hooks, got %d", hook.Count())
	}
}
