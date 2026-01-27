package domain

import "time"

// IdleDuration returns the idle timeout as a duration.
func (s ServerSpec) IdleDuration() time.Duration {
	if s.IdleSeconds <= 0 {
		return 0
	}
	return time.Duration(s.IdleSeconds) * time.Second
}

// DrainTimeout returns the drain timeout as a duration, applying defaults.
func (s ServerSpec) DrainTimeout() time.Duration {
	seconds := s.DrainTimeoutSeconds
	if seconds <= 0 {
		seconds = DefaultDrainTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

// ServerInitRetryBaseDuration returns the base retry delay for server init.
func (c RuntimeConfig) ServerInitRetryBaseDuration() time.Duration {
	seconds := c.ServerInitRetryBaseSeconds
	if seconds <= 0 {
		seconds = DefaultServerInitRetryBaseSeconds
	}
	return time.Duration(seconds) * time.Second
}

// ServerInitRetryMaxDuration returns the max retry delay for server init.
func (c RuntimeConfig) ServerInitRetryMaxDuration() time.Duration {
	seconds := c.ServerInitRetryMaxSeconds
	if seconds <= 0 {
		seconds = DefaultServerInitRetryMaxSeconds
	}
	return time.Duration(seconds) * time.Second
}

// RouteTimeout returns the route timeout duration, defaulting when unset.
func (c RuntimeConfig) RouteTimeout() time.Duration {
	seconds := c.RouteTimeoutSeconds
	if seconds <= 0 {
		seconds = DefaultRouteTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

// PingInterval returns the ping interval duration or zero if disabled.
func (c RuntimeConfig) PingInterval() time.Duration {
	seconds := c.PingIntervalSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// ToolRefreshInterval returns the interval between tool refreshes or zero if disabled.
func (c RuntimeConfig) ToolRefreshInterval() time.Duration {
	seconds := c.ToolRefreshSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// ClientCheckInterval returns the client heartbeat check interval or zero if disabled.
func (c RuntimeConfig) ClientCheckInterval() time.Duration {
	seconds := c.ClientCheckSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// ClientInactiveInterval returns the client inactivity cutoff or zero if disabled.
func (c RuntimeConfig) ClientInactiveInterval() time.Duration {
	seconds := c.ClientInactiveSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// BootstrapTimeout returns the per-server bootstrap timeout, defaulting when unset.
func (c RuntimeConfig) BootstrapTimeout() time.Duration {
	seconds := c.BootstrapTimeoutSeconds
	if seconds <= 0 {
		seconds = DefaultBootstrapTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

// SessionTTLDuration returns the session TTL duration when positive.
func (s ServerSpec) SessionTTLDuration() time.Duration {
	if s.SessionTTLSeconds <= 0 {
		return 0
	}
	return time.Duration(s.SessionTTLSeconds) * time.Second
}

// KeepaliveClientDuration returns the keepalive time duration for RPC clients.
func (c RPCConfig) KeepaliveClientDuration() time.Duration {
	seconds := c.KeepaliveTimeSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// KeepaliveClientTimeout returns the keepalive timeout duration for RPC clients.
func (c RPCConfig) KeepaliveClientTimeout() time.Duration {
	seconds := c.KeepaliveTimeoutSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// KeepaliveServerDuration returns the keepalive time duration for RPC servers.
func (c RPCConfig) KeepaliveServerDuration() time.Duration {
	return c.KeepaliveClientDuration()
}

// KeepaliveServerTimeout returns the keepalive timeout duration for RPC servers.
func (c RPCConfig) KeepaliveServerTimeout() time.Duration {
	return c.KeepaliveClientTimeout()
}
