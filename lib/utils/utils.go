package utils

import (
	"github.com/Sirupsen/logrus"
	"github.com/cenkalti/backoff"
	"time"
)

func GetOrElse(value interface{}, err error) func(defaultValue interface{}) interface{} {
	return func(defaultValue interface{}) interface{} {
		if err != nil {
			return defaultValue
		} else {
			return value
		}
	}
}

const retryBackoffRoundRatio = time.Millisecond / time.Nanosecond

// retry takes any function f: () -> (interface, interface, error) and calls it with exponential backoff.
// If the function succeeds, it returns the values returned by f and the GitHub API response.
// If it continues to fail until a maximum time is reached, the last return values from the function as well
// as a timeout error.
func Retry(log logrus.Entry, timeout time.Duration, f func() (interface{}, interface{}, error)) (interface{}, interface{}, error) {

	var ret interface{}
	var res interface{}

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout

	backoffErr := backoff.RetryNotify(op, b, func(err error, duration time.Duration) {
		// Round to a whole number of milliseconds
		duration /= retryBackoffRoundRatio // Convert nanoseconds to milliseconds
		duration *= retryBackoffRoundRatio // Convert back so it appears correct

		log.Errorf("error performing operation; retrying in %v: %v", duration, err)
	})

	return ret, res, backoffErr
}

// treats nil the same as empty slice.
func SliceStringsEq(a []string, b []string) bool {

	var aa []string
	var bb []string

	if a == nil {
		aa = make([]string, 0)
	} else {
		aa = a
	}

	if b == nil {
		bb = make([]string, 0)
	} else {
		bb = b
	}

	if len(aa) != len(bb) {
		return false
	}

	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}

	return true
}
