package contextvalidation

import "time"

type fixedContextValidationClock struct {
	timestamp time.Time
}

func (clock fixedContextValidationClock) Now() time.Time {
	return clock.timestamp
}
