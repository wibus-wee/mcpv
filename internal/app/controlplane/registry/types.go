package registry

import "time"

type clientState struct {
	pid           int
	tags          []string
	server        string
	specKeys      []string
	lastHeartbeat time.Time
}
