package app

import "mcpv/internal/app/controlplane"

// Version is the semantic version of mcpv, set at build time via -ldflags.
var Version = controlplane.Version

// Build is the git commit hash or build identifier, set at build time via -ldflags.
var Build = controlplane.Build
