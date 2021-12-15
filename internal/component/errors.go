package component

import (
	"fmt"
	"net/http"
)

const ErrNotFound stringError = "not found"

func IsErrNotFound(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(stringError)
	return ok && e == ErrNotFound
}

type stringError string

func (str stringError) Error() string { return string(str) }

type ErrorUnexpectedStatus struct {
	Want, Got int
}

func checkStatus(want, got int) error {
	if want != got {
		return ErrorUnexpectedStatus{
			Want: want, Got: got,
		}
	}
	return nil
}

func (err ErrorUnexpectedStatus) Error() string {
	return fmt.Sprintf("request responded with %s (%d)",
		http.StatusText(err.Got), err.Got,
	)
}
