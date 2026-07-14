package release

import "time"

// ReleaseClock supplies timestamps at explicit release boundaries.
type ReleaseClock interface {
	Now() time.Time
}

type systemReleaseClock struct{}

func (systemReleaseClock) Now() time.Time {
	return time.Now()
}
