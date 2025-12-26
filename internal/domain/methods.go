package domain

func MethodAllowed(caps ServerCapabilities, method string) bool {
	switch method {
	case "ping":
		return true
	case "tools/list", "tools/call":
		return caps.Tools != nil
	case "resources/list", "resources/read", "resources/subscribe", "resources/unsubscribe", "resources/templates/list":
		return caps.Resources != nil
	case "prompts/list", "prompts/get":
		return caps.Prompts != nil
	case "logging/setLevel", "notifications/message":
		return caps.Logging != nil
	case "completion/complete":
		return caps.Completions != nil
	default:
		return false
	}
}
