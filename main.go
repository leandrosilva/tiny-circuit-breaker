package main

import (
	"fmt"
	"time"
)

//
// Helpers ------------------------------------------------
//

func printHead(head string) {
	fmt.Printf("\n###\n%s\n", head)
}

func printResponse(res interface{}, fallbacked bool, err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Printf("CONTENT: %v (fallbacked=%t)\n", res, fallbacked)
	}
}

func printState(st CircuitState) {
	fmt.Printf("--- circuit state (%s) ---\n", st.ToString())
}

func printStateChanged(st CircuitState) {
	fmt.Printf("--- circuit state changed (%s) ---\n", st.ToString())
}

func printTripped(fc int) {
	fmt.Printf("--- circuit tripped (%d failures) ---\n", fc)
}

func printResetted(fc int) {
	fmt.Printf("--- circuit resetted (%d failures) ---\n", fc)
}

func await() {
	fmt.Println("--- awaiting 3 seconds ---")
	time.Sleep(4 * time.Second)
}

//
// Boot ---------------------------------------------------
//

func main() {
	printHead("My Always Health Service")
	cb, _ := createCircuitBreaker(healthService, fallback)
	cb.Settings.OnStateChange = func() {
		printStateChanged(cb.State())
	}
	for i := 0; i < 3; i++ {
		res, fallbacked, err := cb.Call()
		printResponse(res, fallbacked, err)
	}

	printHead("My Always Slow Service")
	cb, _ = createCircuitBreaker(slowService, fallback)
	cb.Settings.OnStateChange = func() {
		printStateChanged(cb.State())
	}
	cb.Settings.OnTrip = func() {
		printTripped(cb.FailureCount)
	}
	for i := 0; i < 10; i++ {
		res, fallbacked, err := cb.Call()
		printResponse(res, fallbacked, err)

		if i == 5 || i == 7 {
			await()
			printState(cb.State())
		}
	}

	printHead("My Intermittently Slow Service")
	cb, _ = createCircuitBreaker(countdownToHealthService, fallback)
	cb.Settings.OnStateChange = func() {
		printStateChanged(cb.State())
	}
	cb.Settings.OnTrip = func() {
		printTripped(cb.FailureCount)
	}
	cb.Settings.OnReset = func() {
		printResetted(cb.FailureCount)
	}
	for i := 0; i < 10; i++ {
		res, fallbacked, err := cb.Call()
		printResponse(res, fallbacked, err)

		if i == 5 || i == 7 {
			await()
			printState(cb.State())
		}
	}
}
