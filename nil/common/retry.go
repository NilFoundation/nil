package common

import (
	"time"
)

func Retry(fn func() error, maxRetry int, startBackoff, maxBackoff time.Duration) {
	for attempt := 0; ; attempt++ {
		if err := fn(); err == nil {
			return
		}

		if attempt == maxRetry-1 {
			return
		}

		time.Sleep(startBackoff)
		if startBackoff < maxBackoff {
			startBackoff *= 2
		}
	}
}
