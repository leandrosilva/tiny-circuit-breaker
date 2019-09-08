# tiny-circuit-breaker

This is a tiny implementation of the [Circuit Breaker](https://en.wikipedia.org/wiki/Circuit_breaker_design_pattern) pattern popularized by Michael Nygard in his book [Release It](https://www.amazon.com/Release-Production-Ready-Software-Pragmatic-Programmers/dp/0978739213), very well documented by [Martin Fowler](https://martinfowler.com/bliki/CircuitBreaker.html), and implemented by a number of frameworks, e.g. [Spring Framework](https://spring.io/guides/gs/circuit-breaker/) using Netflix's [Hystrix](https://github.com/Netflix/Hystrix), but for educational purpose only. It means you should not try to use it in production software. Instead use it to learn the pattern and think how it can be extended and improved. That's why only minimal edge cases are dealt with here and there are lots of comments everywhere in the code.

## Problem

This is a pattern to deal with latency and fault tolerance when writing code to integrate software that runs in different machines, i.e. distributed systems. When building distributed systems the only certainty is that fail will happen at some point in time. There is no way around that but there is a way to put some damage control in place to prevent a humongous cascade of failures down the road.

What happens when a given service that your service depends on gets slow and bottled? Or when it out of nothing starts to throw up some 500s? Your service starts to slow down and waste its time calling a service that are only getting worst, right? That's when a circuit breaker comes in hand.

## Solution

Somewhat paraphasing Mr. Nygard, what you should do is to wrap your remote calls into a component that can circumvent calls when the target service is not healthy. Therefore this technique differs from the retry technique, in that circuit breakers exist to prevent operations rather to reexecute them. And also to trip it down when it times out.

## How to use it

It is very easy to start to play with this circuit breaker. First thing to do is to wrap remote service call in a function with the given signature.

   	func yourMaybeAwesomeService() (interface{}, error) {
        // HTTP, gRPC, or whatever remote call
    }

You may want to provide a fallback function, what is strongly recommended, so in case of a failure you can serve your clients with some cached content or what not. Same signature too.

   	func yourCachedContent() (interface{}, error) {
        // get something from cache
    }

Once that you have your service functions, now you can create the circuit breaker object.

    cb, err := NewCircuitBreaker(CircuitSettings{
		Service:          yourMaybeAwesomeService,
		Fallback:         yourCachedContent,
		Timeout:          2000, // ms
		RetryTimePeriod:  2000, // ms
		FailureThreshold: 10,   // count
		OnStateChange: func() {
			// what ever
		},
		OnTrip: func() {
			// what ever
		},
		OnReset: func() {
			// what ever
		},
	})

And now it is only a matter of make calls to the target service using the circuit breaker object.

	res, fallbacked, err := cb.Call()
    # res        = service content | fallback content | nil
    # fallbacked = true | false
    # err        = error | nil

It is simple like that.

### Sample output

If you run `main.go` one of the examples will give you an output close to this following one:

    ERROR: Service was fallbacked due to error: Error when calling service: Service timed out after 2000 milliseconds
    --- circuit state changed (open) ---
    --- circuit tripped (2 failures) ---
    ERROR: Service was fallbacked due to error: Error when calling service: Service timed out after 2000 milliseconds
    ERROR: Service was fallbacked due to open state
    ERROR: Service was fallbacked due to open state
    ERROR: Service was fallbacked due to open state
    ERROR: Service was fallbacked due to open state
    --- awaiting 3 seconds ---
    --- circuit state (half-open) ---
    --- circuit state changed (open) ---
    --- circuit tripped (7 failures) ---
    ERROR: Service was fallbacked due to error: Error when calling service: Service timed out after 2000 milliseconds
    ERROR: Service was fallbacked due to open state
    --- awaiting 3 seconds ---
    --- circuit state (half-open) ---
    --- circuit state changed (closed) ---
    --- circuit resetted (0 failures) ---
    CONTENT: This is a health fast response (fallbacked=false)
    CONTENT: This is a health fast response (fallbacked=false)

Read it again and try to relate it to the previous snippets.
