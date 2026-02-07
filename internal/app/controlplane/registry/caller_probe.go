package registry

type CallerProbe interface {
	Alive(pid int) bool
}

type OSCallerProbe struct{}

func NewCallerProbe() CallerProbe {
	return OSCallerProbe{}
}

func (OSCallerProbe) Alive(pid int) bool {
	return pidAlive(pid)
}
