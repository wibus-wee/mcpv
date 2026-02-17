package main

type exitError struct {
	code    int
	message string
	silent  bool
}

func (e exitError) Error() string {
	return e.message
}

func exitSilent(code int) error {
	return exitError{code: code, silent: true}
}
