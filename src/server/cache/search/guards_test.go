package search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMustBeEnabled(t *testing.T) {
	require.Error(t, MustBeEnabled(false))
	require.NoError(t, MustBeEnabled(true))
}
