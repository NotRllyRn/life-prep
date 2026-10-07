package main

import "time"

func dailyBriefing(enabled bool, generate func(time.Time) error, now time.Time) error {
	if !enabled {
		return nil
	}
	return generate(now)
}
