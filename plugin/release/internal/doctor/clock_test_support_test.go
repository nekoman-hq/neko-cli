package doctor

import "time"

type fixedReleaseClock struct {
	timestamp time.Time
}

func (clock fixedReleaseClock) Now() time.Time {
	return clock.timestamp
}
