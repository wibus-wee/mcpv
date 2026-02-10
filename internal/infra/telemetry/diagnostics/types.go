package diagnostics

import "time"

// EventPhase describes the lifecycle phase of a diagnostics event.
type EventPhase string

const (
	// PhaseEnter indicates a step has started.
	PhaseEnter EventPhase = "enter"
	// PhaseExit indicates a step has completed successfully.
	PhaseExit EventPhase = "exit"
	// PhaseError indicates a step failed with an error.
	PhaseError EventPhase = "error"
)

const (
	// StepLauncherStart tracks launcher start for stdio servers.
	StepLauncherStart = "launcher_start"
	// StepTransportConnect tracks transport connection.
	StepTransportConnect = "transport_connect"
	// StepInitializeCall tracks initialize call attempts.
	StepInitializeCall = "initialize_call"
	// StepInitializeResponse tracks initialize response validation.
	StepInitializeResponse = "initialize_response"
	// StepNotifyInitialized tracks initialized notification delivery.
	StepNotifyInitialized = "notify_initialized"
	// StepInstanceReady tracks instance ready transition.
	StepInstanceReady = "instance_ready"
	// StepSetMinReady tracks min-ready scheduling calls.
	StepSetMinReady = "set_min_ready"
	// StepSnapshotDone tracks server init snapshot completion.
	StepSnapshotDone = "snapshot_done"
	// StepAcquireFailure tracks acquire failures and capacity diagnostics.
	StepAcquireFailure = "acquire_failure"
)

// Event captures a single diagnostics stage observation.
type Event struct {
	SpecKey    string
	ServerName string
	AttemptID  string
	Step       string
	Phase      EventPhase
	Timestamp  time.Time
	Duration   time.Duration
	Error      string
	Attributes map[string]string
	Sensitive  map[string]string
}

// Probe records diagnostics events in a non-blocking way.
type Probe interface {
	Record(event Event)
}

// NoopProbe ignores all diagnostics events.
type NoopProbe struct{}

func (NoopProbe) Record(Event) {}

// SensitiveProbe reports whether sensitive data collection is enabled.
type SensitiveProbe interface {
	CaptureSensitive() bool
}
