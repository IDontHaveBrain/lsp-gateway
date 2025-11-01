package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSCIPCacheManagerMustBeEnabled(t *testing.T) {
	managerDisabled := &SCIPCacheManager{state: CacheDisabled}
	require.Error(t, managerDisabled.MustBeEnabled())

	managerRunning := &SCIPCacheManager{state: CacheRunning}
	require.NoError(t, managerRunning.MustBeEnabled())
}
