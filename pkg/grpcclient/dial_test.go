package grpcclient_test

import (
	"testing"

	"github.com/farid/billing-service/pkg/grpcclient"
	"github.com/stretchr/testify/require"
)

// TestDialCreatesConnection verifies Dial returns a valid connection.
// Retry behavior is tested via integration tests (see plan verification section).
func TestDialCreatesConnection(t *testing.T) {
	// Dial with invalid address should still create conn (lazy connect)
	conn, err := grpcclient.Dial("localhost:9999")
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer func() { _ = conn.Close() }()
}

// TestDialWithValidAddress verifies connection to a real address.
// This test requires a gRPC server running on localhost:9091 (billing-service).
// Skip if server not available.
func TestDialWithValidAddress(t *testing.T) {
	t.Skip("Integration test - requires billing-service running on :9091")

	conn, err := grpcclient.Dial("localhost:9091")
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer func() { _ = conn.Close() }()
}

