package executor

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/e2etester/test_suite_base"
)

type TestOptions struct {
	Name       string
	images     []string
	testHandle interface{}
}

func NewTestOptions(name string, i []string, th interface{}) *TestOptions {
	return &TestOptions{
		Name:       name,
		images:     i,
		testHandle: th,
	}
}

func (t *TestOptions) GetImages() []string {
	return t.images
}

func (t *TestOptions) GetTestHandle() interface{} {
	return t.testHandle
}

type ITestExecutor struct {
	testSets []*TestOptions
}

var executor *ITestExecutor

func GetITestExecutor() *ITestExecutor {
	if executor == nil {
		tOptions := []*TestOptions{}
		executor = &ITestExecutor{testSets: tOptions}
	}
	return executor
}

func (e *ITestExecutor) AddTestOptions(t *TestOptions) error {
	if t == nil {
		return fmt.Errorf("cannot add empty option")
	}
	e.testSets = append(e.testSets, t)
	return nil
}

func (e *ITestExecutor) GetTestOptions() []*TestOptions {
	return e.testSets
}

func (e *ITestExecutor) Execute(options *TestOptions) {
	if isRunningOnGCE() {
		run(options.GetTestHandle())
		return
	}
	test_suite_base.Prepare(options.Name, options.GetImages(), options.GetTestHandle())
}

func run(testHandle interface{}) {
	err := test_suite_base.RunAllTests(testHandle)
	if err != nil {
		fmt.Printf("TEST-FAILURE: test failed: %v\n", err)
	} else {
		fmt.Printf("TEST-SUCCESS: test succeeded\n")
	}
}

func isRunningOnGCE() bool {
	metadataServer := "http://metadata.google.internal/computeMetadata/"
	resp, err := http.Get(metadataServer)
	if err != nil {
		return !strings.Contains(err.Error(), "no such host")
	}
	return resp.StatusCode == http.StatusOK
}
