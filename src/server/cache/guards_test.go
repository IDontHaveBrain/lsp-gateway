package cache

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSCIPCacheManagerWithEnabledGuard(t *testing.T) {
	manager := &SCIPCacheManager{enabled: true}

	value, err := manager.WithEnabledGuard(func() (interface{}, error) {
		return "ok", nil
	})

	require.NoError(t, err)
	require.Equal(t, "ok", value)
}

func TestSCIPCacheManagerWithEnabledGuardDisabled(t *testing.T) {
	manager := &SCIPCacheManager{enabled: false}
	called := false

	value, err := manager.WithEnabledGuard(func() (interface{}, error) {
		called = true
		return "ok", nil
	})

	require.NoError(t, err)
	require.False(t, called)
	require.Nil(t, value)
}

func TestSCIPCacheManagerWithEnabledGuardError(t *testing.T) {
	manager := &SCIPCacheManager{enabled: true}
	expectedErr := errors.New("boom")

	_, err := manager.WithEnabledGuard(func() (interface{}, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSCIPCacheManagerWithIndexResultDisabled(t *testing.T) {
	manager := &SCIPCacheManager{enabled: false}

	result, err := manager.WithIndexResult("test", func() (*IndexResult, error) {
		return &IndexResult{}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "test", result.Type)
	require.NotNil(t, result.Metadata)
	require.False(t, result.Metadata.CacheEnabled)
}

func TestSCIPCacheManagerWithSliceResult(t *testing.T) {
	manager := &SCIPCacheManager{enabled: false}

	result, err := manager.WithSliceResult(func() ([]interface{}, error) {
		return []interface{}{"value"}, nil
	})

	require.NoError(t, err)
	require.Empty(t, result)
}

func TestSCIPCacheManagerMustBeEnabled(t *testing.T) {
	manager := &SCIPCacheManager{enabled: false}
	require.Error(t, manager.MustBeEnabled())

	manager.enabled = true
	require.NoError(t, manager.MustBeEnabled())
}

func TestSCIPCacheManagerWithManagerGuard(t *testing.T) {
	manager := &SCIPCacheManager{enabled: true, started: true}

	value, ready, err := manager.WithManagerGuard(func() (interface{}, bool, error) {
		return time.Second, true, nil
	})

	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, time.Second, value)
}
