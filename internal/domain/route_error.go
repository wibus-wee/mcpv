package domain

import (
	"errors"
	"fmt"
)

type RouteStage string

const (
	RouteStageDecode   RouteStage = "decode"
	RouteStageValidate RouteStage = "validate"
	RouteStageAcquire  RouteStage = "acquire"
	RouteStageCall     RouteStage = "call"
)

type RouteError struct {
	Stage RouteStage
	Err   error
}

func (e *RouteError) Error() string {
	return fmt.Sprintf("%s: %v", e.Stage, e.Err)
}

func (e *RouteError) Unwrap() error {
	return e.Err
}

func NewRouteError(stage RouteStage, err error) error {
	if err == nil {
		return nil
	}
	var routeErr *RouteError
	if errors.As(err, &routeErr) {
		return err
	}
	return &RouteError{Stage: stage, Err: err}
}

func RouteStageFrom(err error) (RouteStage, bool) {
	var routeErr *RouteError
	if errors.As(err, &routeErr) {
		return routeErr.Stage, true
	}
	return "", false
}
