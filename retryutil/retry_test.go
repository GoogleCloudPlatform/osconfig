package retryutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetrySleep(t *testing.T) {
	tests := []struct {
		base               int
		extra              int
		expectedLowerBound time.Duration
		expectedUpperBound time.Duration
	}{
		{base: 1, extra: 0, expectedLowerBound: 1 * time.Second, expectedUpperBound: 2 * time.Second},
		{base: 2, extra: 0, expectedLowerBound: 4 * time.Second, expectedUpperBound: 6 * time.Second},
		{base: 3, extra: 0, expectedLowerBound: 9 * time.Second, expectedUpperBound: 12 * time.Second},

		{base: 1, extra: 5, expectedLowerBound: 6 * time.Second, expectedUpperBound: 12 * time.Second},
		{base: 2, extra: 5, expectedLowerBound: 14 * time.Second, expectedUpperBound: 21 * time.Second},
		{base: 3, extra: 5, expectedLowerBound: 24 * time.Second, expectedUpperBound: 32 * time.Second},

		{base: 1, extra: 10, expectedLowerBound: 11 * time.Second, expectedUpperBound: 22 * time.Second},
		{base: 2, extra: 10, expectedLowerBound: 24 * time.Second, expectedUpperBound: 36 * time.Second},
		{base: 3, extra: 10, expectedLowerBound: 39 * time.Second, expectedUpperBound: 52 * time.Second},
	}

	// Run each test case n times as RetrySleep have randomized nature.
	n := 100

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test case %d", i), func(t *testing.T) {
			for i := 0; i < n; i++ {
				rd := RetrySleep(tt.base, tt.extra)
				if rd < tt.expectedLowerBound || rd > tt.expectedUpperBound {
					t.Errorf("unexpected sleep duration, expected range [%s, %s] got %s", tt.expectedLowerBound, tt.expectedUpperBound, rd)
				}
			}
		})
	}
}

func TestRetryFunc(t *testing.T) {
	tests := []struct {
		name                 string
		maxRetryTime         time.Duration
		expectedToFailTimes  int
		failWith             error
		expectedError        error
		funcCalledLowerBound int
		funcCalledUpperBound int
	}{
		{
			name:                 "Function does not fail",
			maxRetryTime:         time.Minute,
			expectedToFailTimes:  0,
			failWith:             nil,
			expectedError:        nil,
			funcCalledLowerBound: 1,
			funcCalledUpperBound: 1,
		},
		{
			name:                 "Function does fail, retry does not work",
			maxRetryTime:         time.Second,
			expectedToFailTimes:  5,
			failWith:             fmt.Errorf("failure"),
			expectedError:        fmt.Errorf("failure"),
			funcCalledLowerBound: 1,
			funcCalledUpperBound: 2,
		},
		{
			name:                 "Function does fail, retry does work",
			maxRetryTime:         time.Minute,
			expectedToFailTimes:  5,
			failWith:             fmt.Errorf("failure"),
			expectedError:        nil,
			funcCalledLowerBound: 5,
			funcCalledUpperBound: 5,
		},
	}

	currentSleeper = noOpSleeper{} // Avoid calling time.Sleep to speed up tests

	description := "test"
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, count := callsCollector(tt.expectedToFailTimes, tt.failWith)

			err := RetryFunc(ctx, tt.maxRetryTime, description, f)
			if safeString(err) != safeString(tt.expectedError) {
				t.Errorf("unexpected error, exepcted %q, got %q", safeString(tt.expectedError), safeString(err))
			}

			if *count < tt.funcCalledLowerBound || *count > tt.funcCalledUpperBound {
				t.Errorf("unexpected function calls count, expected range [%d, %d], got %d", tt.funcCalledLowerBound, tt.funcCalledUpperBound, *count)
			}
		})
	}
}

func TestRetryAPICall(t *testing.T) {
	tests := []struct {
		name                 string
		maxRetryTime         time.Duration
		callsCollector       func(int, error) (func() error, *int)
		expectedToFailTimes  int
		failWith             error
		expectedError        error
		funcCalledLowerBound int
		funcCalledUpperBound int
	}{
		{
			name:                 "Function does not fail",
			maxRetryTime:         time.Minute,
			expectedToFailTimes:  0,
			failWith:             nil,
			expectedError:        nil,
			funcCalledLowerBound: 1,
			funcCalledUpperBound: 1,
		},
		{
			name:                 "Function fail with non API error",
			maxRetryTime:         time.Second,
			expectedToFailTimes:  5,
			failWith:             fmt.Errorf("failure"),
			expectedError:        fmt.Errorf("failure"),
			funcCalledLowerBound: 1,
			funcCalledUpperBound: 1,
		},
		{
			name:                 "Function fail with non retriable API error",
			maxRetryTime:         time.Minute,
			expectedToFailTimes:  5,
			failWith:             status.Error(codes.InvalidArgument, "invalid"),
			expectedError:        fmt.Errorf("code: \"InvalidArgument\", message: \"invalid\", details: []"),
			funcCalledLowerBound: 1,
			funcCalledUpperBound: 1,
		},
		{
			name:                 "Function fail with retriable API error, retry does not help",
			maxRetryTime:         2 * time.Minute,
			expectedToFailTimes:  10,
			failWith:             status.Error(codes.DeadlineExceeded, "invalid"),
			expectedError:        fmt.Errorf("code: \"DeadlineExceeded\", message: \"invalid\", details: []"),
			funcCalledLowerBound: 6,
			funcCalledUpperBound: 7,
		},
		{
			name:                 "Function fail with retriable API error, retry does help",
			maxRetryTime:         2 * time.Minute,
			expectedToFailTimes:  3,
			failWith:             status.Error(codes.DeadlineExceeded, "invalid"),
			expectedError:        nil,
			funcCalledLowerBound: 3,
			funcCalledUpperBound: 3,
		},
		{
			name:                 "Function fail with ResourceExhausted error, additional time between retries",
			maxRetryTime:         2 * time.Minute,
			expectedToFailTimes:  10,
			failWith:             status.Error(codes.ResourceExhausted, "invalid"),
			expectedError:        fmt.Errorf("code: \"ResourceExhausted\", message: \"invalid\", details: []"),
			funcCalledLowerBound: 3,
			funcCalledUpperBound: 4,
		},
	}

	currentSleeper = noOpSleeper{} // Avoid calling time.Sleep to speed up tests

	description := "test"
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, count := callsCollector(tt.expectedToFailTimes, tt.failWith)

			err := RetryAPICall(ctx, tt.maxRetryTime, description, f)
			if safeString(err) != safeString(tt.expectedError) {
				t.Errorf("unexpected error, exepcted %q, got %q", safeString(tt.expectedError), safeString(err))
			}

			if *count < tt.funcCalledLowerBound || *count > tt.funcCalledUpperBound {
				t.Errorf("unexpected function calls count, expected range [%d, %d], got %d", tt.funcCalledLowerBound, tt.funcCalledUpperBound, *count)
			}
		})
	}
}

func Test_defaultSleeper(t *testing.T) {
	sleeper := defaultSleeper{}

	timeToSleep := 200 * time.Millisecond
	before := time.Now()

	sleeper.Sleep(timeToSleep)

	after := time.Now()
	elapsed := after.Sub(before)

	// Tolerate 10% difference to reduce test flakiness.
	maxTimeDifference := timeToSleep / 10
	if abs(elapsed.Milliseconds()-timeToSleep.Milliseconds()) > maxTimeDifference.Milliseconds() {
		t.Errorf("sleeper.Sleep, elapsed time %s bigger than expected %s", elapsed, timeToSleep)
	}
}

func abs(d int64) int64 {
	if d < 0 {
		return d * -1
	}

	return d
}

func safeString(err error) string {
	if err == nil {
		return "<nil>"
	}

	return err.Error()
}

func callsCollector(expectedToFailTimes int, failWith error) (func() error, *int) {
	var c int
	return func() error {
		c++
		if expectedToFailTimes <= c {
			return nil
		}

		return failWith
	}, &c
}

type noOpSleeper struct{}

func (noOpSleeper) Sleep(d time.Duration) { /*no op*/ }
