// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import "time"

type RateLimiter struct {
	Burst           int
	Frequency       int
	BaseDelay       time.Duration
	FailureMaxDelay time.Duration
}
