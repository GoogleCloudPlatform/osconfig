package error

import "fmt"

// ErrorCollector is used to combine
// a list of errors together into a
// single error
type ErrorCollector []error

// Collect collects all the errors to be combined
// together
func (c *ErrorCollector) Collect(e error) {
	*c = append(*c, e)
}

// Error returns number of errors collected and combined
// error from collected errors
func (c *ErrorCollector) Error() (int, error) {
	if len(*c) == 0 {
		return 0, nil
	}

	err := "Errors:\n"
	for _, e := range *c {
		err += fmt.Sprintf("%s\n", e.Error())
	}
	return len(*c), fmt.Errorf(err)
}
