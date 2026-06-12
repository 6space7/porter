package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type FailureLimiter interface {
	Allow(key string) bool
	RecordFailure(key string)
	Reset(key string)
}

type MemoryFailureLimiter struct {
	mu        sync.Mutex
	records   map[string]failureRecord
	threshold int
	baseDelay time.Duration
	maxDelay  time.Duration
	now       func() time.Time
}

type failureRecord struct {
	failures     int
	blockedUntil time.Time
}

func NewMemoryFailureLimiter(threshold int, baseDelay, maxDelay time.Duration) *MemoryFailureLimiter {
	if threshold < 1 {
		threshold = 5
	}
	if baseDelay <= 0 {
		baseDelay = time.Second
	}
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}
	return &MemoryFailureLimiter{
		records:   map[string]failureRecord{},
		threshold: threshold,
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
		now:       time.Now,
	}
}

func NewDefaultFailureLimiter() *MemoryFailureLimiter {
	return NewMemoryFailureLimiter(5, time.Second, time.Minute)
}

func (limiter *MemoryFailureLimiter) Allow(key string) bool {
	if limiter == nil {
		return true
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	record := limiter.records[key]
	return !limiter.now().Before(record.blockedUntil)
}

func (limiter *MemoryFailureLimiter) RecordFailure(key string) {
	if limiter == nil {
		return
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	record := limiter.records[key]
	record.failures++
	if record.failures >= limiter.threshold {
		delay := limiter.baseDelay
		for i := 0; i < record.failures-limiter.threshold; i++ {
			delay *= 2
			if delay >= limiter.maxDelay {
				delay = limiter.maxDelay
				break
			}
		}
		record.blockedUntil = limiter.now().Add(delay)
	}
	limiter.records[key] = record
}

func (limiter *MemoryFailureLimiter) Reset(key string) {
	if limiter == nil {
		return
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.records, key)
}

func clientIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
