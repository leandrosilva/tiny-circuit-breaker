package main

import (
	"fmt"
	"time"
)

// Default value for missing settings on CircuitBreak creation
const (
	DefautTimeout           time.Duration = 2000
	DefaultRetryTimePeriod  time.Duration = 3000
	DefautlFailureThreshold int           = 2
)

// CircuitState flags the state of the circuit
type CircuitState int

const (
	_ CircuitState = iota
	// IsClosed is default state to when everything is right with
	// the service.
	IsClosed
	// IsHalfOpen is the state that signs we should periodically
	// make calls to the service in order to check if it is right again.
	IsHalfOpen
	// IsOpen is the state when the server is down, so we should
	// use cached data or, in absense of that, fail as soon as possible.
	IsOpen
)

// ToString of CircuitState type
func (s CircuitState) ToString() string {
	switch s {
	case IsClosed:
		return "closed"
	case IsHalfOpen:
		return "half-open"
	case IsOpen:
		return "open"
	default:
		return "invalid"
	}
}

// CircuitEvent is used for callback purposes
type CircuitEvent func()

// CircuitSettings is the spec to build a CircuitBreaker instance
type CircuitSettings struct {
	// Target service
	Service Callable
	// Fallback when service is unhealth
	Fallback Callable
	// Request timeout in milliseconds
	Timeout time.Duration
	// Grace time in milliseconds to wait before a new call to the service
	RetryTimePeriod time.Duration
	// How many fails should we tolerate
	FailureThreshold int
	// It happens when the circuit trips
	OnTrip CircuitEvent
	// It happens when the circuit get closed again
	OnReset CircuitEvent
	// It happens whenever state changes
	OnStateChange CircuitEvent
}

// Callable is the actual call to a service or it might as well be a fallback
type Callable func() (interface{}, error)

type callableResponse struct {
	Content interface{}
	Error   error
}

// CallingError is an error that occurs on a callable action
type CallingError struct {
	Cause error
}

func (e *CallingError) Error() string {
	return fmt.Sprintf("Error when calling service: %s", e.Cause.Error())
}

// CircuitBreaker object itself
type CircuitBreaker struct {
	// Spec to follow
	Settings CircuitSettings
	// It is the last time the service failed
	LastFailureTime time.Time
	// How many time the service failed
	FailureCount int
	// A record of all errors that happenend since last time it was cool
	FailureRecord []string
}

// NewCircuitBreaker builds a circuit breaker from a settings spec
func NewCircuitBreaker(settings CircuitSettings) (*CircuitBreaker, error) {
	if settings.Service == nil {
		return nil, fmt.Errorf("You must provide a service to be called")
	}

	if settings.Timeout == 0 {
		settings.Timeout = DefautTimeout
	}
	if settings.RetryTimePeriod == 0 {
		settings.RetryTimePeriod = DefaultRetryTimePeriod
	}
	if settings.FailureThreshold == 0 {
		settings.FailureThreshold = DefautlFailureThreshold
	}

	cb := &CircuitBreaker{
		Settings:        settings,
		LastFailureTime: time.Time{},
		FailureCount:    0,
		FailureRecord:   []string{},
	}
	return cb, nil
}

// State reflects the most up to date state of circuit
func (cb *CircuitBreaker) State() CircuitState {
	if cb.FailureCount >= cb.Settings.FailureThreshold {
		// When it has already faild too much, we should do something
		gracePeriod := time.Now().Sub(cb.LastFailureTime) * time.Millisecond
		if gracePeriod > cb.Settings.RetryTimePeriod {
			// In this case, we can give it a chance
			return IsHalfOpen
		}
		// No change is given, keep it open for now yet
		return IsOpen
	}
	// While failure count doesn't reach failure threashold, keep it closed
	return IsClosed
}

// Call is the circuit break safe call to a service.
// Returns:
// - Service actual response content;
// - True if relying on fallback, False otherwise;
// - An error or nil otherwise.
func (cb *CircuitBreaker) Call() (interface{}, bool, error) {
	// What is the current state pre call to service
	preState := cb.State()

	res, fallbacked, err := cb.selectiveCall(preState)
	if fallbacked {
		// When we get a fallback, it means we got an error at some point
		cb.recordFailure(err)
	} else {
		// If we're not dealing with a fallback, it means everything is good
		// and we can reset circuit state
		cb.resetState()
	}

	// After all we look at state again because it might be require for a change
	newState := cb.State()
	cb.notifyState(preState, newState)

	return res, fallbacked, err
}

func (cb *CircuitBreaker) selectiveCall(state CircuitState) (interface{}, bool, error) {
	switch state {
	case IsOpen:
		// When open, use the fallback function, we might rely on cache or something
		res, fallbacked, err := cb.mayCallFallback()
		if err != nil {
			return res, fallbacked, fmt.Errorf("Service was fallbacked due to open state but failed too: %s", err.Error())
		}
		return res, fallbacked, fmt.Errorf("Service was fallbacked due to open state")
	case IsHalfOpen:
		// When it is this state we call give it a one chance to go
		fallthrough
	case IsClosed:
		// This function calls the service within a timeout restrict time
		res, err := cb.callService()
		if err != nil {
			// In case of any error, we go for a possible fallback
			res, fallbacked, fberr := cb.mayCallFallback()
			if fallbacked {
				if fberr != nil {
					// Even the fallback may get an error
					return res, fallbacked, fmt.Errorf("Service was fallbacked due to error but failed too: %s: %s", fberr.Error(), err.Error())
				}
				return res, fallbacked, fmt.Errorf("Service was fallbacked due to error: %s", err.Error())
			}
			return res, false, err
		}
		// Damn! We made it. Everything is fresh and cool
		return res, false, err
	default:
		return nil, false, fmt.Errorf("Unknown state")
	}
}

func (cb *CircuitBreaker) callService() (interface{}, error) {
	responseChannel := make(chan callableResponse, 1)

	go func() {
		res, err := cb.Settings.Service()
		responseChannel <- callableResponse{res, err}
	}()

	select {
	case res := <-responseChannel:
		if res.Error != nil {
			return nil, &CallingError{res.Error}
		}
		if res.Content == nil {
			err := fmt.Errorf("Service respond is nil")
			return nil, &CallingError{err}
		}
		return res.Content, nil
	case <-time.After(time.Duration(cb.Settings.Timeout) * time.Millisecond):
		err := fmt.Errorf("Service timed out after %d milliseconds", cb.Settings.Timeout)
		return nil, &CallingError{err}
	}
}

func (cb *CircuitBreaker) mayCallFallback() (interface{}, bool, error) {
	if cb.Settings.Fallback == nil {
		return nil, false, nil
	}
	// So ok, we have a fallback and we're going to rely on it
	res, err := cb.Settings.Fallback()
	return res, true, err
}

func (cb *CircuitBreaker) resetState() {
	cb.FailureCount = 0
	cb.FailureRecord = []string{}
	cb.LastFailureTime = time.Time{}
}

func (cb *CircuitBreaker) recordFailure(err error) {
	cb.FailureCount = cb.FailureCount + 1
	cb.LastFailureTime = time.Now()
	if err == nil {
		err = fmt.Errorf("Service is relying on fallback")
	}
	cb.FailureRecord = append(cb.FailureRecord, err.Error())
}

func (cb *CircuitBreaker) notifyState(preState, newState CircuitState) {
	// Anytime state changes
	if newState != preState {
		// We notify it generally
		if cb.Settings.OnStateChange != nil {
			cb.Settings.OnStateChange()
		}
		// And specifically
		switch newState {
		case IsOpen:
			if cb.Settings.OnTrip != nil {
				cb.Settings.OnTrip()
			}
		case IsClosed:
			if cb.Settings.OnReset != nil {
				cb.Settings.OnReset()
			}
		}
	}
}
