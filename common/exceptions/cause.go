package exceptions

type causeError struct {
	message string
	cause   error
}

func (e *causeError) Error() (str string) {
	defer func() { //karing
		if err := recover(); err != nil {
			str = e.message + ": recover from panic"
		}
	}()
	if e.cause == nil { //karing
		return e.message
	}
	return e.message + ": " + e.cause.Error()
}

func (e *causeError) Unwrap() error {
	return e.cause
}
