package timeutil

import (
	"sync"
	"time"
)

var (
	// If testHookTimeNow is non-nil, then it overrides time.Now() calls
	// within this package.
	testHookTimeNow func() time.Time
)

// StubTestHookTimeNow stubs all `timeutil.Now()` use.
//
// It sets up current time to given now time.
// It returns time shifter function that shifts current time by given
// duration.
// It returns cleanup function that MUST be called after test execution.
//
// NOTE: tests using this function MUST not be called concurrently.
func StubTestHookTimeNow(now time.Time) (shifter func(time.Duration), cleanup func()) {
	orig := testHookTimeNow
	cleanup = func() {
		testHookTimeNow = orig
	}
	var mu sync.Mutex
	testHookTimeNow = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	shifter = func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(d)
	}
	return
}
