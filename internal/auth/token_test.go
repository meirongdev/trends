package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomTokenAndHash(t *testing.T) {
	a, err := RandomToken(32)
	require.NoError(t, err)
	b, err := RandomToken(32)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.GreaterOrEqual(t, len(a), 32)

	h1 := HashToken(a)
	require.Equal(t, h1, HashToken(a))
	require.NotEqual(t, a, h1)
	require.Len(t, h1, 64)
}
