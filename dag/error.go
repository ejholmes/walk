package dag

import (
	"fmt"
	"strings"
)

// MultiError is an error type to track multiple errors. This is used to
// accumulate errors in cases and return them as a single "error".
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	es := e.Errors
	if len(es) == 1 {
		return fmt.Sprintf("1 error occurred:\n\n* %s", es[0])
	}

	points := make([]string, len(es))
	for i, err := range es {
		points[i] = fmt.Sprintf("* %s", err)
	}

	return fmt.Sprintf(
		"%d errors occurred:\n\n%s",
		len(es), strings.Join(points, "\n"))
}

// appendError is a helper function that will append more errors
// onto a MultiError in order to create a larger multi-error.
func appendErrors(err error, errs ...error) *MultiError {
	switch err := err.(type) {
	case *MultiError:
		// Typed nils can reach here, so initialize if we are nil
		if err == nil {
			err = new(MultiError)
		}

		// Go through each error and flatten
		for _, e := range errs {
			switch e := e.(type) {
			case *MultiError:
				if e != nil {
					err.Errors = append(err.Errors, e.Errors...)
				}
			default:
				if e != nil {
					err.Errors = append(err.Errors, e)
				}
			}
		}

		return err
	default:
		newErrs := make([]error, 0, len(errs)+1)
		if err != nil {
			newErrs = append(newErrs, err)
		}
		newErrs = append(newErrs, errs...)

		return appendErrors(&MultiError{}, newErrs...)
	}
}
