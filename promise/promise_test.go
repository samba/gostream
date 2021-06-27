package promise

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func Assert(t *testing.T, val bool, err string, args ...interface{}) {
	if !val {
		t.Errorf(err, args...)
		t.Fail()
	}
}

func TestPromiseChain_withRecovery(t *testing.T) {

	did_reset := false
	base := 200

	result := NewPromise(func(resolve Resolver, reject Rejector) error {
		go func() {
			select {
			case <-time.After(300 * time.Millisecond):
				resolve(base)
			case <-time.After(200 * time.Millisecond):
				reject(errors.New("FOILED!"))
			}
		}()
		return nil
	}).Then(func(i Unknown) Unknown {
		return (i).(int) * 2
	}, nil).Catch(func(e error) Unknown {
		t.Logf("Caught error: %s", e)
		t.Log("Recovering with value (5) for next hop")
		did_reset = true
		return 5 // overrides with alternate resolution
	}).Then(func(i Unknown) Unknown {
		return 13 * (i).(int)
	}, nil)

	// Wait() blocks on internal Channel() channels, so no need for time.After() here
	res, err := result.Wait()

	Assert(t, result.GetStatus() == "Resolved", "Promise state was not Resolved, found %v", result.GetStatus())

	// NB: did_reset gets changed async during the flow of the promises
	// therefore this must happen *after* Wait()
	if did_reset {
		base = 5
	} else {
		base = base * 2
	}

	Assert(t, (13*base) == res, "Promise result value: %d != %v", (13 * base), res)

	Assert(t, err == nil, "Promise yielded an unexpected error: %s", err)

}

func TestPromise_Race(t *testing.T) {

	start := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(50 * time.Millisecond)
				reject(errors.New("FAIL"))
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				resolve("One")
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				resolve("Two")
			}()

			return nil
		}),
	}

	Race(start...).Then(func(u Unknown) Unknown {
		Assert(t, u == "Two", "Race produced unexpected result: %v", u)
		return nil
	}, nil)

}

func TestPromise_All(t *testing.T) {
	// one of these rejects, to verify correct failure behavior
	willfail := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(50 * time.Millisecond)
				reject(errors.New("FAIL"))
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				resolve("One")
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				resolve("Two")
			}()

			return nil
		}),
	}

	// This case will fail
	All(willfail...).Then(func(u Unknown) Unknown {
		Assert(t, false, "All() passed when it should have failed [%u]", u)
		return nil
	}, func(e error) Unknown {
		Assert(t, e.Error() == "FAIL", "All() failed with an unexpected error: %s", e)
		return nil
	})

	willpass := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(50 * time.Millisecond)
				resolve("Zero")
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				resolve("One")
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				resolve("Two")
			}()

			return nil
		}),
	}

	// This case will pass
	All(willpass...).Then(func(u Unknown) Unknown {
		values, ok := (u).([]string)

		if !ok {
			t.Fatalf("Could not coerce the combined result of all promises")
		}

		for i, val := range []string{"Zero", "One", "Two"} {
			Assert(t, values[i] == val, "All() produced a mismatching value: %v != %v", values[i], val)
		}

		return nil
	}, func(e error) Unknown {
		Assert(t, true, "All() failed with an unexpected error: %s", e)
		return nil
	})
}

func TestPromise_Any(t *testing.T) {
	willpass := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(50 * time.Millisecond)
				reject(fmt.Errorf("error: %d", 0))
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				resolve("One")
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				resolve("Two")
			}()

			return nil
		}),
	}

	// This case will pass
	Any(willpass...).Then(func(u Unknown) Unknown {
		value, ok := (u).(string)

		if !ok {
			t.Fatalf("Could not coerce the result of a promise")
		}

		Assert(t, value == "Two", "Any() produced a mismatching value: %v != %v", value, "Two")

		return nil
	}, func(e error) Unknown {
		Assert(t, true, "Any() failed with an unexpected error: %s", e)
		return nil
	})

	willfail := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(50 * time.Millisecond)
				reject(fmt.Errorf("error: %d", 1))
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				reject(fmt.Errorf("error: %d", 2))
			}()

			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				reject(fmt.Errorf("error: %d", 3))
			}()

			return nil
		}),
	}

	// This case will fail, with the (chronologically) last error above
	Any(willfail...).Then(func(u Unknown) Unknown {
		Assert(t, false, "Any() passed when it should have failed [%u]", u)
		return nil
	}, func(e error) Unknown {
		t.Logf("(expected) failure: %s", e)
		multi, ok := (e).(*MultiPromiseError)

		if ok && (multi != nil) {
			for i, o := range (*multi).Outcomes {
				t.Logf("(expected) error: %v", o)
				Assert(t, o.Reason.Error() == fmt.Sprintf("error: %d", i+1),
					"Error did not match the expected format: %s", o.Reason)
			}
		}

		return nil
	})

}

func TestPromise_AllSettled(t *testing.T) {

	promises := []Promise{

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(300 * time.Millisecond)
				resolve(1)
			}()
			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(200 * time.Millisecond)
				resolve(2)
			}()
			return nil
		}),

		NewPromise(func(resolve Resolver, reject Rejector) error {
			go func() {
				<-time.After(100 * time.Millisecond)
				reject(errors.New("FOILED!"))
			}()
			return nil
		}),
	}

	AllSettled(promises...).Then(func(u Unknown) Unknown {
		result, ok := (u).([]*PromiseOutcome)

		if !(ok && (result != nil)) {
			t.Errorf("Could not coerce promise outcomes for AllSettled()")
		}

		Assert(t, len(promises) == len(result), "Results did not match promise input for AllSettled()")

		Assert(t, result[0].Reason == nil, "First promise yielded unexpected error: %s", result[0].Reason)
		Assert(t, result[1].Reason == nil, "Second promise yielded unexpected error: %s", result[1].Reason)
		Assert(t, result[2].Reason != nil, "Third promise should have failed, but did not: %v", result[2].Result)

		Assert(t, result[0].Result == 1, "First result did not match expected value: %v", result[0].Result)
		Assert(t, result[1].Result == 2, "Second result did not match expected value: %v", result[1].Result)

		return nil
	}, func(e error) Unknown {
		Assert(t, true, "AllSettled() failed with an unexpected error: %s", e)
		return nil
	})

}
