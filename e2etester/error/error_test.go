package error

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorCollectorError(t *testing.T) {
	collector := new(ErrorCollector)
	collector.Collect(fmt.Errorf("error 1"))
	collector.Collect(fmt.Errorf("error 2"))
	count, err := collector.Error()
	if count != 2 {
		t.Errorf("Unexpected number of Errors: (%d) wanted (%d)", count, 2)
	}
	if !strings.Contains(err.Error(), "error 1") {
		t.Errorf("Unexpected error: (%s)! wanted(%s)", err.Error(), "error 1")
	}

	if !strings.Contains(err.Error(), "error 2") {
		t.Errorf("Unexpected error: (%s)! wanted(%s)", err.Error(), "error 2")
	}
}

func TestErrorCollectorEmptyCollector(t *testing.T) {
	collector := new(ErrorCollector)
	count, err := collector.Error()
	if count != 0 {
		t.Errorf("Unexpected number of Errors: (%d) wanted (%d)", count, 0)
	}

	if err != nil {
		t.Errorf("Unexpected error: (+%v)! wanted(nil)", err)
	}
}
