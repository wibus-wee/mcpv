package domain

import "encoding/json"

const (
	ErrCodeURLElicitationRequired = -32042
)

// ProtocolError captures JSON-RPC error details for propagation.
type ProtocolError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *ProtocolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}
