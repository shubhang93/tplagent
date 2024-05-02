package fatal

type Error struct {
	err error
}

func (e Error) Error() string {
	return e.err.Error()
}

func NewError(err error) error {
	return Error{err: err}
}

func (e Error) Fatal() bool {
	return true
}

func Is(err error) bool {
	fe, ok := err.(interface{ Fatal() bool })
	return ok && fe.Fatal()
}
