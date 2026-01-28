package devices

import (
	"fmt"
	"sync"

	"github.com/mobile-next/mobilecli/utils"
)

// ShutdownHook manages graceful shutdown of application resources.
// It allows registering cleanup functions that will be called during
// application shutdown (SIGINT/SIGTERM).
type ShutdownHook struct {
	mu    sync.RWMutex
	hooks []namedHook
}

type namedHook struct {
	name string
	fn   func() error
}

// NewShutdownHook creates a new shutdown hook registry
func NewShutdownHook() *ShutdownHook {
	return &ShutdownHook{
		hooks: make([]namedHook, 0),
	}
}

// Register adds a cleanup function to be called during shutdown.
// The name parameter is used for logging and error reporting.
// Cleanup functions are called in the order they were registered.
func (s *ShutdownHook) Register(name string, cleanupFn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, namedHook{name: name, fn: cleanupFn})
	utils.Verbose("Registered shutdown hook: %s", name)
}

// Shutdown executes all registered cleanup functions.
// Returns an error if any cleanup function fails, but continues
// executing remaining hooks to ensure best-effort cleanup.
func (s *ShutdownHook) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.hooks) == 0 {
		return nil
	}

	utils.Verbose("Executing %d shutdown hook(s)", len(s.hooks))
	var errs []error

	for _, hook := range s.hooks {
		utils.Verbose("Running shutdown hook: %s", hook.name)
		if err := hook.fn(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", hook.name, err))
			utils.Verbose("Shutdown hook %s failed: %v", hook.name, err)
		}
	}

	// clear hooks after shutdown
	s.hooks = make([]namedHook, 0)

	if len(errs) > 0 {
		return fmt.Errorf("shutdown failed with %d error(s): %v", len(errs), errs)
	}

	utils.Verbose("All shutdown hooks completed successfully")
	return nil
}

// Count returns the number of registered hooks (useful for testing)
func (s *ShutdownHook) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.hooks)
}
