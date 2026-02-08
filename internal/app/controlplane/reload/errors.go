package reload

import "errors"

type ApplyError struct {
	Stage string
	Err   error
}

func (e ApplyError) Error() string {
	if e.Stage == "" {
		return e.Err.Error()
	}
	return e.Stage + ": " + e.Err.Error()
}

func (e ApplyError) Unwrap() error {
	return e.Err
}

func WrapStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	var applyErr ApplyError
	if errors.As(err, &applyErr) {
		return err
	}
	return ApplyError{Stage: stage, Err: err}
}

func FailureStage(err error) string {
	var applyErr ApplyError
	if errors.As(err, &applyErr) && applyErr.Stage != "" {
		return applyErr.Stage
	}
	return "unknown"
}
