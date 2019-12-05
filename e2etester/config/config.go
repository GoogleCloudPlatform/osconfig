package config

import (
	"flag"
	"time"
)

const (
	DefaultParallelCount = 5
	TimeFormat           = time.RFC3339
)

var (
	Oauth         = flag.String("oauth", "", "path to oauth json file")
	Project       = flag.String("project", "", "comma separated list of project that can be used for tests")
	Images        = flag.String("images", "", "comma separated list of images to run tests for")
	Zone          = flag.String("zone", "", "zone to use for tests")
	PrintTests    = flag.Bool("print", false, "print out the parsed test cases for debugging")
	Validate      = flag.Bool("validate", false, "validate all the test cases and exit")
	Ce            = flag.String("compute_endpoint_override", "", "API endpoint to override default")
	Filter        = flag.String("filter", "", "test name filter")
	OutPath       = flag.String("out_path", "junit.xml", "junit xml path")
	ParallelCount = flag.Int("parallel_count", 0, "TestParallelCount")
)

func init() {
	flag.Parse()
}
