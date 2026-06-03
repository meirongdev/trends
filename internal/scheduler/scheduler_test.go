package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistersJobsWithoutError(t *testing.T) {
	called := 0
	s, err := New(
		Job{Spec: "@every 1h", Run: func() { called++ }},
		Job{Spec: "@every 2h", Run: func() { called++ }},
	)
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, 2, s.EntryCount())
}

func TestNewRejectsBadSpec(t *testing.T) {
	_, err := New(Job{Spec: "not-a-cron", Run: func() {}})
	require.Error(t, err)
}
