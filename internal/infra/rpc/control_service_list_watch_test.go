package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"mcpv/internal/domain"
)

func TestGuardedWatch_DefaultSkipWhenLastETagMatches(t *testing.T) {
	updates := make(chan string, 1)
	updates <- "v1"
	close(updates)

	sent := 0
	err := guardedWatch(guardedWatchPlan[string, *emptypb.Empty]{
		ctx:      context.Background(),
		guard:    &governanceGuard{},
		request:  domain.GovernanceRequest{},
		op:       "test",
		lastETag: "v1",
		subscribe: func(context.Context) (<-chan string, error) {
			return updates, nil
		},
		etag: func(value string) string {
			return value
		},
		toProto: func(string) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		},
		mapError: func(err error) error {
			return err
		},
		send: func(*emptypb.Empty) error {
			sent++
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, 0, sent)
}

func TestGuardedWatch_DefaultSendsInitialSnapshot(t *testing.T) {
	updates := make(chan string, 1)
	updates <- "v1"
	close(updates)

	sent := 0
	err := guardedWatch(guardedWatchPlan[string, *emptypb.Empty]{
		ctx:      context.Background(),
		guard:    &governanceGuard{},
		request:  domain.GovernanceRequest{},
		op:       "test",
		lastETag: "",
		subscribe: func(context.Context) (<-chan string, error) {
			return updates, nil
		},
		etag: func(value string) string {
			return value
		},
		toProto: func(string) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		},
		mapError: func(err error) error {
			return err
		},
		send: func(*emptypb.Empty) error {
			sent++
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, sent)
}
