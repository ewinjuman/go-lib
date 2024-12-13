package http_v2

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreaker struct {
	mu                   sync.RWMutex
	state                string
	failureCount         int
	successCount         int
	lastFailureTime      time.Time
	failureThreshold     int
	recoveryTimeout      time.Duration
	totalRequestCount    int
	failureRateThreshold float64
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:                "CLOSED",
		failureThreshold:     5,               // Jumlah kegagalan maksimum sebelum membuka sirkuit
		recoveryTimeout:      1 * time.Minute, // Waktu tunggu sebelum mencoba kembali
		failureRateThreshold: 0.5,             // 50% kegagalan akan membuka sirkuit
	}
}

func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Jika sirkuit terbuka, periksa apakah sudah waktunya recovery
	if cb.state == "OPEN" {
		if time.Since(cb.lastFailureTime) < cb.recoveryTimeout {
			return errors.New("circuit is open")
		}
		cb.state = "HALF_OPEN"
	}

	return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequestCount++
	cb.successCount++

	// Reset jika berhasil
	if cb.state == "HALF_OPEN" {
		cb.state = "CLOSED"
	}
	cb.failureCount = 0
}

func (cb *CircuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequestCount++
	cb.failureCount++

	// Hitung tingkat kegagalan
	failureRate := float64(cb.failureCount) / float64(cb.totalRequestCount)

	// Jika jumlah kegagalan atau tingkat kegagalan melebihi ambang batas
	if cb.failureCount >= cb.failureThreshold || failureRate >= cb.failureRateThreshold {
		cb.state = "OPEN"
		cb.lastFailureTime = time.Now()
	}
}
