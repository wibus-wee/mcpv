package domain

import "time"

// InstanceOptions provides initial values for a new Instance.
type InstanceOptions struct {
	ID         string
	Spec       ServerSpec
	SpecKey    string
	State      InstanceState
	Conn       Conn
	SpawnedAt  time.Time
	LastActive time.Time
}

// NewInstance constructs a new instance with the provided options.
func NewInstance(opts InstanceOptions) *Instance {
	inst := &Instance{
		id:         opts.ID,
		spec:       opts.Spec,
		specKey:    opts.SpecKey,
		state:      opts.State,
		conn:       opts.Conn,
		spawnedAt:  opts.SpawnedAt,
		lastActive: opts.LastActive,
	}
	if inst.lastActive.IsZero() {
		inst.lastActive = time.Now()
	}
	return inst
}

// ID returns the instance ID.
func (i *Instance) ID() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.id
}

// Spec returns the server spec for this instance.
func (i *Instance) Spec() ServerSpec {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.spec
}

// SpecKey returns the spec key for this instance.
func (i *Instance) SpecKey() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.specKey
}

// State returns the instance state.
func (i *Instance) State() InstanceState {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.state
}

// SetState updates the instance state.
func (i *Instance) SetState(state InstanceState) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state = state
}

// BusyCount returns the current busy count.
func (i *Instance) BusyCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.busyCount
}

// SetBusyCount sets the busy count.
func (i *Instance) SetBusyCount(count int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.busyCount = count
}

// IncBusyCount increments busy count and returns the new value.
func (i *Instance) IncBusyCount() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.busyCount++
	return i.busyCount
}

// DecBusyCount decrements busy count when possible and returns the new value.
func (i *Instance) DecBusyCount() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.busyCount > 0 {
		i.busyCount--
	}
	return i.busyCount
}

// LastActive returns the last active timestamp.
func (i *Instance) LastActive() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lastActive
}

// SetLastActive updates the last active timestamp.
func (i *Instance) SetLastActive(at time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lastActive = at
}

// SpawnedAt returns the spawn timestamp.
func (i *Instance) SpawnedAt() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.spawnedAt
}

// SetSpawnedAt updates the spawn timestamp.
func (i *Instance) SetSpawnedAt(at time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.spawnedAt = at
}

// HandshakedAt returns the handshake timestamp.
func (i *Instance) HandshakedAt() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.handshakedAt
}

// SetHandshakedAt updates the handshake timestamp.
func (i *Instance) SetHandshakedAt(at time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.handshakedAt = at
}

// LastHeartbeatAt returns the last heartbeat timestamp.
func (i *Instance) LastHeartbeatAt() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lastHeartbeatAt
}

// SetLastHeartbeatAt updates the last heartbeat timestamp.
func (i *Instance) SetLastHeartbeatAt(at time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lastHeartbeatAt = at
}

// StickyKey returns the sticky routing key.
func (i *Instance) StickyKey() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.stickyKey
}

// SetStickyKey updates the sticky routing key.
func (i *Instance) SetStickyKey(key string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stickyKey = key
}

// Conn returns the connection.
func (i *Instance) Conn() Conn {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.conn
}

// SetConn updates the connection.
func (i *Instance) SetConn(conn Conn) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.conn = conn
}

// Capabilities returns the cached capabilities.
func (i *Instance) Capabilities() ServerCapabilities {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.capabilities
}

// SetCapabilities updates the cached capabilities.
func (i *Instance) SetCapabilities(caps ServerCapabilities) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.capabilities = caps
}

// LastStartCause returns a copy of the last start cause.
func (i *Instance) LastStartCause() *StartCause {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return CloneStartCause(i.lastStartCause)
}

// SetLastStartCause updates the last start cause.
func (i *Instance) SetLastStartCause(cause *StartCause) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lastStartCause = CloneStartCause(cause)
}

// Info returns a snapshot of instance info.
func (i *Instance) Info() InstanceInfo {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return InstanceInfo{
		ID:              i.id,
		State:           i.state,
		BusyCount:       i.busyCount,
		LastActive:      i.lastActive,
		SpawnedAt:       i.spawnedAt,
		HandshakedAt:    i.handshakedAt,
		LastHeartbeatAt: i.lastHeartbeatAt,
		LastStartCause:  CloneStartCause(i.lastStartCause),
	}
}
