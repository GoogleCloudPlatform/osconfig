package test_suite_base

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2etester/config"
)

func Prepare(suiteName string, images []string, testHandle interface{}) {
	ctx := context.Background()

	var regex *regexp.Regexp
	if *config.Filter != "" {
		var err error
		regex, err = regexp.Compile(*config.Filter)
		if err != nil {
			fmt.Println("-filter flag not valid:", err)
			os.Exit(1)
		}
	}

	ts, err := CreateTestSuite(suiteName, images, testHandle)
	if err != nil {
		log.Fatalln("test case creation error:", err)
	}

	if ts == nil {
		// If filters resulted in no possible tests..?
		return
	}

	errors := make(chan error, len(ts.Tests))
	// Retry failed locks 2x as many tests in the test case.
	retries := len(ts.Tests) * 2
	if len(ts.Tests) == 0 {
		fmt.Println("[TestRunner] Nothing to do")
		return
	}

	if *config.PrintTests {
		for n, t := range ts.Tests {
			if t.W == nil {
				continue
			}
			fmt.Printf("[TestRunner] Printing test case %q\n", n)
			t.W.Print(ctx)
		}
		CheckError(errors)
		return
	}

	if *config.Validate {
		for n, t := range ts.Tests {
			if t.W == nil {
				continue
			}
			fmt.Printf("[TestRunner] Validating test case %q\n", t.W.Name)
			if err := t.W.Validate(ctx); err != nil {
				errors <- fmt.Errorf("Error validating test case %s: %v", n, err)
			}
		}
		CheckError(errors)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*config.Oauth), 0770); err != nil {
		log.Fatal(err)
	}

	junit := &junitTestSuite{Name: ts.Name, Tests: len(ts.Tests)}
	tests := make(chan *test, len(ts.Tests))
	var wg sync.WaitGroup
	for i := 0; i < ts.TestParallelCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for test := range tests {
				tc := &junitTestCase{Classname: ts.Name, ID: test.testCase.id, Name: test.name}
				junit.mx.Lock()
				junit.TestCase = append(junit.TestCase, tc)
				junit.mx.Unlock()

				if test.testCase.W == nil {
					junit.mx.Lock()
					junit.Skipped++
					junit.mx.Unlock()
					tc.Skipped = &junitSkipped{Message: fmt.Sprintf("Test does not match filter: %q", regex.String())}
					continue
				}

				runTestCase(ctx, test, tc, errors, retries)
			}
		}()
	}

	start := time.Now()
	for n, t := range ts.Tests {
		tests <- &test{name: n, testCase: t}
	}
	close(tests)
	wg.Wait()

	fmt.Printf("[TestRunner] Creating junit xml file: %q\n", *config.OutPath)
	junit.Time = time.Since(start).Seconds()
	d, err := xml.MarshalIndent(junit, "  ", "   ")
	if err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(*config.OutPath, d, 0644); err != nil {
		log.Fatal(err)
	}

	CheckError(errors)
	fmt.Println("[TestRunner] All test cases completed successfully.")
}

func runTestCase(ctx context.Context, test *test, tc *junitTestCase, errors chan error, retries int) {
	if err := test.testCase.W.PopulateClients(ctx); err != nil {
		errors <- fmt.Errorf("%s: %v", tc.Name, err)
		tc.Failure = &junitFailure{FailMessage: err.Error(), FailType: "Error"}
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		select {
		case <-c:
			fmt.Printf("\nCtrl-C caught, sending cancel signal to %q...\n", test.name)
			close(test.testCase.W.Cancel)
			err := fmt.Errorf("test case %q was canceled", test.name)
			errors <- err
			tc.Failure = &junitFailure{FailMessage: err.Error(), FailType: "Canceled"}
		case <-test.testCase.W.Cancel:
		}
	}()

	project := test.testCase.W.Project
	client := test.testCase.W.ComputeClient
	key := test.testCase.W.ID()
	var lock string
	var err error
	if test.testCase.CustomProjectLock != "" {
		for i := 0; i < retries; i++ {
			lock, err = customProjectWriteLock(client, project, allowedChars.ReplaceAllString(test.testCase.CustomProjectLock, "_"), key, test.testCase.timeout)
			if err == nil {
				break
			}
		}
		if err != nil {
			errors <- err
			return
		}
	} else if test.testCase.ProjectLock {
		for i := 0; i < retries; i++ {
			lock, err = projectWriteLock(client, project, key, test.testCase.timeout)
			if err == nil {
				break
			}
		}
		if err != nil {
			errors <- err
			return
		}
	} else {
		for i := 0; i < retries; i++ {
			lock, err = projectReadLock(client, project, key, test.testCase.timeout)
			if err == nil {
				break
			}
		}
		if err != nil {
			errors <- err
			return
		}
	}
	defer func() {
		for i := 0; i < retries; i++ {
			err := projectUnlock(client, project, lock)
			if err == nil {
				break
			}
		}
		if err != nil {
			fmt.Printf("[TestRunner] Test %q: Error unlocking project: %v\n", test.name, err)
		}
	}()

	select {
	case <-test.testCase.W.Cancel:
		return
	default:
	}

	start := time.Now()
	fmt.Printf("[TestRunner] Running test case %q\n", tc.Name)
	if err := test.testCase.W.Run(ctx); err != nil {
		errors <- fmt.Errorf("%s: %v", tc.Name, err)
		tc.Failure = &junitFailure{FailMessage: err.Error(), FailType: "Failure"}
	}
	tc.Time = time.Since(start).Seconds()
	tc.SystemOut = test.testCase.logger.Buf.String()
	fmt.Printf("[TestRunner] Test case %q finished\n", tc.Name)
}
