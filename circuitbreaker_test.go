package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestErrorOnCreationWithoutProvideAService(t *testing.T) {
	cb, err := createCircuitBreakerWithNoService()
	assert.NotNil(t, err)
	assert.Nil(t, cb)
}

func TestNoErrorOnCreationWithoutProvideAFallback(t *testing.T) {
	cb, err := createCircuitBreakerWithNoFallback(healthService)
	assert.Nil(t, err)
	assert.NotNil(t, cb)
}

func TestServiceIsHealth(t *testing.T) {
	cb, _ := createCircuitBreaker(healthService, fallback)
	res, fallbacked, err := cb.Call()
	assert.Nil(t, err)
	assert.Equal(t, false, fallbacked)
	assert.Equal(t, healthServiceContent, res)
	assert.Equal(t, IsClosed, cb.State())
}

func TestServiceIsSlow(t *testing.T) {
	cb, _ := createCircuitBreaker(slowService, fallback)
	res, fallbacked, err := cb.Call()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
	assert.True(t, fallbacked)
	assert.Equal(t, fallbackContent, res)
	assert.Equal(t, IsClosed, cb.State())
}

func TestServiceIsSlowButThereIsNoFallback(t *testing.T) {
	cb, _ := createCircuitBreakerWithNoFallback(slowService)
	res, fallbacked, err := cb.Call()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), serviceTimedOutMessage)
	assert.Equal(t, false, fallbacked)
	assert.Nil(t, res)
	assert.Equal(t, IsClosed, cb.State())
}

func TestCircuitShouldOpenWhenReachThreashold(t *testing.T) {
	cb, _ := createCircuitBreaker(slowService, fallback)
	assert.Equal(t, IsClosed, cb.State())

	cb.Settings.OnTrip = func() {
		assert.Equal(t, IsOpen, cb.State())
	}
	cb.Settings.OnReset = func() {
		assert.Fail(t, "Should not reset")
	}

	for i := 0; i < cb.Settings.FailureThreshold; i++ {
		assert.Equal(t, IsClosed, cb.State())
		//
		res, fallbacked, err := cb.Call()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
		assert.True(t, fallbacked)
		assert.Contains(t, fallbackContent, res)
	}
	assert.Equal(t, IsOpen, cb.State())
}

func TestCircuitShouldHalfOpenAfterRetryTimePeriod(t *testing.T) {
	cb, _ := createCircuitBreaker(slowService, fallback)
	assert.Equal(t, IsClosed, cb.State())

	for i := 0; i < cb.Settings.FailureThreshold; i++ {
		// still closed while inside failure threashold
		assert.Equal(t, IsClosed, cb.State())
		//
		res, fallbacked, err := cb.Call()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
		assert.True(t, fallbacked)
		assert.Contains(t, fallbackContent, res)
	}
	// should trip after reach failure threashold
	assert.Equal(t, IsOpen, cb.State())

	// wait something to benefit from a half-open state due to retry time period
	time.Sleep(2 * time.Second)
	assert.Equal(t, IsHalfOpen, cb.State())

	res, fallbacked, err := cb.Call()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
	assert.True(t, fallbacked)
	assert.Contains(t, fallbackContent, res)
	assert.Equal(t, IsOpen, cb.State())
}

func TestServiceIsAlwaysSlow(t *testing.T) {
	cb, _ := createCircuitBreaker(slowService, fallback)
	assert.Equal(t, IsClosed, cb.State())

	for i := 0; i < cb.Settings.FailureThreshold; i++ {
		// still closed while inside failure threashold
		assert.Equal(t, IsClosed, cb.State())
		//
		res, fallbacked, err := cb.Call()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
		assert.True(t, fallbacked)
		assert.Contains(t, fallbackContent, res)
	}
	// should trip after reach failure threashold
	assert.Equal(t, IsOpen, cb.State())

	for i := 0; i < 2; i++ {
		// should be in open state all the way
		res, fallbacked, err := cb.Call()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fallbackDueToOpenStateMessage)
		assert.True(t, fallbacked)
		assert.Contains(t, fallbackContent, res)
		assert.Equal(t, IsOpen, cb.State())
	}

	// wait something to benefit from a half-open state due to retry time period
	time.Sleep(2 * time.Second)
	assert.Equal(t, IsHalfOpen, cb.State())

	res, fallbacked, err := cb.Call()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fallbackDueToErrorMessage)
	assert.True(t, fallbacked)
	assert.Contains(t, fallbackContent, res)
	assert.Equal(t, IsOpen, cb.State())

	res, fallbacked, err = cb.Call()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fallbackDueToOpenStateMessage)
	assert.True(t, fallbacked)
	assert.Contains(t, fallbackContent, res)
	assert.Equal(t, IsOpen, cb.State())
}

func TestServiceIsIntermittentlySlow(t *testing.T) {
	cb, _ := createCircuitBreaker(countdownToHealthService, fallback)
	assert.Equal(t, IsClosed, cb.State())

	for i := countdownToHealth; i > 0; i-- {
		res, fallbacked, err := cb.Call()
		assert.NotNil(t, err)
		assert.True(t, fallbacked)
		assert.Contains(t, fallbackContent, res)
	}
	// should trip after reach failure threashold
	assert.Equal(t, IsOpen, cb.State())
	assert.Equal(t, 1, countdownToHealth)

	// wait something to benefit from a half-open state due to retry time period
	time.Sleep(2 * time.Second)
	assert.Equal(t, IsHalfOpen, cb.State())

	// will fail again
	res, fallbacked, err := cb.Call()
	assert.NotNil(t, err)
	assert.True(t, fallbacked)
	assert.Contains(t, fallbackContent, res)
	assert.Equal(t, IsOpen, cb.State())
	assert.Equal(t, 0, countdownToHealth)

	// wait a little bit more
	time.Sleep(2 * time.Second)
	assert.Equal(t, IsHalfOpen, cb.State())

	// countdonw is over and service should be health now
	cb.Settings.OnReset = func() {
		assert.Equal(t, IsClosed, cb.State())
	}

	for i := 0; i < 3; i++ {
		res, fallbacked, err := cb.Call()
		assert.Nil(t, err)
		assert.Equal(t, false, fallbacked)
		assert.Equal(t, countdownToHealthContent, res)
		assert.Equal(t, IsClosed, cb.State())
	}
}
