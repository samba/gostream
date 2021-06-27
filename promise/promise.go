package promise

import (
	"fmt"
	"strings"
)

type Unknown interface{}
type Resolver func(Unknown) Unknown
type Rejector func(error) Unknown

type PromiseHandler func(Resolver, Rejector) error

const (
	PromisePending = iota
	PromiseResolved
	PromiseRejected
)

var (
	PromiseStatusName = []string{"Pending", "Resolved", "Rejected"}
)

type PromiseError struct {
	description string
}

type MultiPromiseError struct {
	Promises    []Promise
	Description string
}

func (e *PromiseError) Error() string {
	return fmt.Sprintf("Promise error: %s", e.description)
}

func NewPromiseError(text string) *PromiseError {
	return &PromiseError{
		description: text,
	}
}

func (e *MultiPromiseError) Error() string {
	message := []string{}
	for _, e := range e.Outcomes() {
		if e.Reason != nil {
			message = append(message, e.Reason.Error())
		}
	}
	return fmt.Sprintf("%s: %s", e.Description, strings.Join(message, "; "))
}

func NewMultiPromiseError(description string, promises []Promise) *MultiPromiseError {
	return &MultiPromiseError{
		Promises:    promises,
		Description: description,
	}
}

func (m *MultiPromiseError) Outcomes() []*PromiseOutcome {
	return getPromiseOutcomes(m.Promises)
}

func getPromiseOutcomes(proms []Promise) []*PromiseOutcome {
	result := make([]*PromiseOutcome, len(proms))
	for i, p := range proms {
		result[i] = p.Outcome()
	}
	return result
}

type Thenable interface {
	Then(Resolver, Rejector) Promise
}

type Promise interface {
	Then(Resolver, Rejector) Promise
	Catch(Rejector) Promise
	GetStatus() string
	Channel() (<-chan Unknown, <-chan error)
	Wait() (Unknown, error)
	makeError(string) error
	Outcome() *PromiseOutcome
}

type aPromise struct {
	status    uint
	result    Unknown
	reject    error
	callbacks []func(uint, Unknown, error)
}

func (p *aPromise) Outcome() *PromiseOutcome {
	return &PromiseOutcome{
		Status: p.GetStatus(),
		Result: p.result,
		Reason: p.reject,
	}
}

// Race produces a Promise that will resolve or reject with the value of the first the input promise that resolves or rejects.
func Race(proms ...Promise) Promise {
	return NewPromise(func(resolve Resolver, reject Rejector) error {
		for _, p := range proms {
			p.Then(resolve, reject)
		}
		return nil
	})
}

// All produces a Promise that resovles with the results of _all_ the input promises.
func All(proms ...Promise) Promise {
	return NewPromise(func(resolve Resolver, reject Rejector) error {

		count := len(proms)
		count_success := 0
		_results := make([]Unknown, count)

		make_resolver := func(i int, p Promise, resolve Resolver) Resolver {
			return func(u Unknown) Unknown {
				_results[i] = u
				count_success += 1
				if count == count_success {
					return resolve(_results)
				}
				return nil
			}
		}

		// register on each incoming promise
		for i, p := range proms {
			p.Then(make_resolver(i, p, resolve), reject)
		}

		return nil
	})
}

// Any produces a Promise that resolves with the first input promise that fulfills (not account for rejections).
func Any(proms ...Promise) Promise {
	return NewPromise(func(resolve Resolver, reject Rejector) error {
		failures := 0
		total := len(proms)
		for index, prom := range proms {
			func(i int, prom Promise) {
				prom.Then(resolve, func(e error) Unknown {
					failures += 1
					if failures == total {
						reject(NewMultiPromiseError("all promises failed", proms))
					}
					return nil
				})
			}(index, prom)
		}
		return nil
	})
}

type PromiseOutcome struct {
	Status string
	Result Unknown
	Reason error
}

// AllSettled produces a promise which resolves when all input promises are settled (fulfilled or rejected).
func AllSettled(proms ...Promise) Promise {

	return NewPromise(func(resolve Resolver, reject Rejector) error {
		total := len(proms)
		settled := 0

		for i, p := range proms {
			func(index int, prom Promise) {
				prom.Then(func(u Unknown) Unknown {
					settled += 1
					if settled == total {
						resolve(getPromiseOutcomes(proms))
					}
					return nil
				}, func(e error) Unknown {
					settled += 1
					if settled == total {
						resolve(getPromiseOutcomes(proms))
					}
					return nil
				})
			}(i, p)
		}
		return nil
	})
}

// Resolve produces a Promise that is immediately resolved with the input value.
func Resolve(val Unknown) Promise {
	return NewPromise(func(r1 Resolver, r2 Rejector) error {
		r1(val)
		return nil
	})
}

// Reject produces a Promise that is immediately rejected with the input error.
func Reject(err error) Promise {
	return NewPromise(func(r1 Resolver, r2 Rejector) error {
		return err
	})
}

func (p *aPromise) makeError(message string) error {
	return NewPromiseError(fmt.Sprintf("%s (status: %s)", message, p.GetStatus()))
}

func (p *aPromise) Then(onsuccess Resolver, onfail Rejector) Promise {
	return NewPromise(func(resolve Resolver, reject Rejector) error {

		handle := func(status uint, val Unknown, err error) {

			// Handles error's from handlers panicing.
			defer func() {
				r := recover()
				err, ok := (r).(error)
				if ok && (err != nil) {
					reject(err)
				}
			}()

			switch status {
			case PromiseRejected: // the parent promise failed
				if onfail != nil {
					res := onfail(err)
					err, ok := (res).(error)
					if ok && (err != nil) {
						reject(err)
					}
					if res != nil {
						resolve(res) // non-error returns from rejection are treated as recovery
					}
				} else {
					reject(err) // pass through to derivative promise

				}

			case PromiseResolved: // the parent promise fulfilled
				if onsuccess != nil {
					res := onsuccess(val)

					err, ok := res.(error)
					if ok && (err != nil) {
						reject(err)
					}

					prom, ok := res.(Promise)
					if ok && (prom != nil) {
						prom.Then(resolve, reject)
					}

					// any other values should be regarded as derivative results
					resolve(res)

				} else {
					resolve(val) // pass through to derivative promise
				}

			}
		}

		switch p.status {
		case PromisePending:
			// Enqueue the promise for pending values
			p.callbacks = append(p.callbacks, handle)
		default:
			// Execute the promise with existing values
			go handle(p.status, p.result, p.reject)
		}

		return nil
	})
}

func (p *aPromise) Catch(catch Rejector) Promise {
	return p.Then(nil, catch)
}

func (p *aPromise) GetStatus() string {
	return PromiseStatusName[p.status]
}

func (p *aPromise) String() string {
	return fmt.Sprintf("<Promise: %s> (%v, %v)", p.GetStatus(), p.result, p.reject)
}

func (p *aPromise) Channel() (<-chan Unknown, <-chan error) {
	result := make(chan Unknown, 1)
	errout := make(chan error, 1)
	p.Then(func(i Unknown) Unknown {
		result <- i
		close(errout)
		close(result)
		return nil
	}, func(e error) Unknown {
		errout <- e
		close(result)
		close(errout)
		return nil
	})
	return result, errout
}

func (p *aPromise) Wait() (Unknown, error) {
	result, errout := p.Channel()
	select {
	case res := <-result:
		return res, nil
	case err := <-errout:
		return nil, err
	}
}

func NewPromise(handler PromiseHandler) Promise {
	prom := aPromise{
		status:    PromisePending,
		reject:    nil,
		result:    nil,
		callbacks: make([]func(uint, Unknown, error), 0),
	}

	var resolve Resolver
	var reject Rejector

	resolve = func(val Unknown) Unknown {
		then, ok := (val).(Thenable)
		if ok {

			defer func() { // If the incoming promise errors, pass through rejection
				r := recover()
				err, _ := r.(error)
				if err != nil {
					reject(err)
				}
			}()

			then.Then(resolve, reject)
		} else if prom.status == PromisePending {
			prom.status = PromiseResolved
			prom.result = val
			for _, han := range prom.callbacks {
				go han(PromiseResolved, val, nil)
			}
			// truncate
			prom.callbacks = prom.callbacks[:0]
		}
		return nil
	}

	reject = func(err error) Unknown {
		if prom.status == PromisePending {
			prom.status = PromiseRejected
			prom.reject = err
			for _, han := range prom.callbacks {
				go han(PromiseRejected, nil, err)
			}
			// truncate
			prom.callbacks = prom.callbacks[:0]
		}
		return nil
	}

	e := handler(resolve, reject)

	if e != nil {
		reject(e)
	}

	return &prom
}
