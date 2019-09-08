package main

import (
	"time"
)

var serviceTimedOutMessage = "Service timed out"
var fallbackDueToOpenStateMessage = "Service was fallbacked due to open state"
var fallbackDueToErrorMessage = "Service was fallbacked due to error"

func createCircuitBreaker(service Callable, fallback Callable) (*CircuitBreaker, error) {
	return NewCircuitBreaker(CircuitSettings{
		Service:          service,
		Fallback:         fallback,
		Timeout:          DefautTimeout,
		RetryTimePeriod:  DefaultRetryTimePeriod,
		FailureThreshold: DefautlFailureThreshold,
	})
}

func createCircuitBreakerWithNoFallback(service Callable) (*CircuitBreaker, error) {
	return createCircuitBreaker(service, nil)
}

func createCircuitBreakerWithNoService() (*CircuitBreaker, error) {
	return createCircuitBreaker(nil, nil)
}

// Fallback
var fallbackContent = "Relying on a fallback cached content"

func fallback() (interface{}, error) {
	return fallbackContent, nil
}

// Health
var healthServiceContent = "A health service gives a fast response"

func healthService() (interface{}, error) {
	return healthServiceContent, nil
}

// Slow
func slowService() (interface{}, error) {
	time.Sleep(5 * time.Minute)
	return "This is a veeery slooow response", nil
}

// Slow then fast
var countdownToHealth = 3
var countdownToHealthContent = "This is a health fast response"

func countdownToHealthService() (interface{}, error) {
	if countdownToHealth > 0 {
		countdownToHealth = countdownToHealth - 1
		time.Sleep(1 * time.Minute)
		return "This is a slow response", nil
	}
	return countdownToHealthContent, nil
}
