package reload

import "context"

type Step struct {
	Name     string
	Apply    func(context.Context) error
	Rollback func(context.Context) error
}
