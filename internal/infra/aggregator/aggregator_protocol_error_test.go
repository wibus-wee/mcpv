package aggregator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestDecodeToolResultURLElicitationRequired(t *testing.T) {
	raw := json.RawMessage(`{"jsonrpc":"2.0","id":1,"error":{"code":-32042,"message":"need info","data":{"elicitations":[]}}}`)

	_, err := decodeToolResult(raw)
	require.Error(t, err)

	var protoErr *domain.ProtocolError
	require.ErrorAs(t, err, &protoErr)
	require.Equal(t, int64(domain.ErrCodeURLElicitationRequired), protoErr.Code)
	require.Equal(t, "need info", protoErr.Message)
}
