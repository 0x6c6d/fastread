package state

import "time"

// Clock abstracts time for tests.
type Clock interface{ Now() time.Time }

// SystemClock is the real clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }
