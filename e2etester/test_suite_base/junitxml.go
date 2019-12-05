package test_suite_base

import (
	"encoding/xml"
	"sync"
)

type junitTestSuite struct {
	mx sync.Mutex

	XMLName  xml.Name `xml:"testsuite"`
	Name     string   `xml:"name,attr"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Errors   int      `xml:"errors,attr"`
	Disabled int      `xml:"disabled,attr"`
	Skipped  int      `xml:"skipped,attr"`
	Time     float64  `xml:"time,attr"`

	TestCase []*junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Classname string        `xml:"classname,attr"`
	ID        string        `xml:"id,attr"`
	Name      string        `xml:"name,attr"`
	Time      float64       `xml:"time,attr"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

type junitFailure struct {
	FailMessage string `xml:",chardata"`
	FailType    string `xml:"type,attr"`
}
