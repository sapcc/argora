package controller

import "time"

type RateLimiter struct {
	Burst           int
	Frequency       int
	BaseDelay       time.Duration
	FailureMaxDelay time.Duration
}
