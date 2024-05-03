package fatal

import "fmt"

type Error struct {
	err error
}

func (e Error) Error() string {
	return e.err.Error()
}

func (e Error) Unwrap() error {
	return e.err
}

func NewError(err error) error {
	return Error{err: fmt.Errorf("%w", err)}
}

func (e Error) Fatal() bool {
	return true
}

func Is(err error) bool {
	fe, ok := err.(interface{ Fatal() bool })
	return ok && fe.Fatal()
}
