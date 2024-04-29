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
