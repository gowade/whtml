package whtml

import (
	"fmt"
)

// Error represents a syntax error.
type Error struct {
	Message string
	Pos     Pos
}

// Error returns the formatted string error message.
func (e *Error) Error() string {
	return sfmt("%v:%v: %v", e.Pos.Line+1, e.Pos.Char, e.Message)
}

// ErrorList represents a list of syntax errors.
type ErrorList []error

// Error returns the formatted string error message.
func (a ErrorList) Error() string {
	switch len(a) {
	case 0:
		return "no errors"
	case 1:
		return a[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", a[0], len(a)-1)
}
