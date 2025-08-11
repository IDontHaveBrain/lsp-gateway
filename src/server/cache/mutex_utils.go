package cache

import "sync"

// WithReadLock executes a function while holding a read lock on the provided RWMutex.
// Returns the result of the function execution.
func WithReadLock(mu *sync.RWMutex, fn func() interface{}) interface{} {
	mu.RLock()
	defer mu.RUnlock()
	return fn()
}

// WithWriteLock executes a function while holding a write lock on the provided RWMutex.
// Returns the result of the function execution.
func WithWriteLock(mu *sync.RWMutex, fn func() interface{}) interface{} {
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// WithReadLockError executes a function while holding a read lock and returns both result and error.
func WithReadLockError(mu *sync.RWMutex, fn func() (interface{}, error)) (interface{}, error) {
	mu.RLock()
	defer mu.RUnlock()
	return fn()
}

// WithWriteLockError executes a function while holding a write lock and returns both result and error.
func WithWriteLockError(mu *sync.RWMutex, fn func() (interface{}, error)) (interface{}, error) {
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// WithReadLockVoid executes a function while holding a read lock without returning a value.
func WithReadLockVoid(mu *sync.RWMutex, fn func()) {
	mu.RLock()
	defer mu.RUnlock()
	fn()
}

// WithWriteLockVoid executes a function while holding a write lock without returning a value.
func WithWriteLockVoid(mu *sync.RWMutex, fn func()) {
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// WithIndexReadLock is a convenience method for the cache manager to execute functions with index read lock.
func (m *SCIPCacheManager) WithIndexReadLock(fn func() interface{}) interface{} {
	return WithReadLock(&m.indexMu, fn)
}

// WithIndexWriteLock is a convenience method for the cache manager to execute functions with index write lock.
func (m *SCIPCacheManager) WithIndexWriteLock(fn func() interface{}) interface{} {
	return WithWriteLock(&m.indexMu, fn)
}

// WithIndexReadLockError is a convenience method for the cache manager to execute functions with index read lock that return errors.
func (m *SCIPCacheManager) WithIndexReadLockError(fn func() (interface{}, error)) (interface{}, error) {
	return WithReadLockError(&m.indexMu, fn)
}

// WithIndexWriteLockError is a convenience method for the cache manager to execute functions with index write lock that return errors.
func (m *SCIPCacheManager) WithIndexWriteLockError(fn func() (interface{}, error)) (interface{}, error) {
	return WithWriteLockError(&m.indexMu, fn)
}

// WithCacheReadLock is a convenience method for the cache manager to execute functions with cache read lock.
func (m *SCIPCacheManager) WithCacheReadLock(fn func() interface{}) interface{} {
	return WithReadLock(&m.mu, fn)
}

// WithCacheWriteLock is a convenience method for the cache manager to execute functions with cache write lock.
func (m *SCIPCacheManager) WithCacheWriteLock(fn func() interface{}) interface{} {
	return WithWriteLock(&m.mu, fn)
}

// WithCacheReadLockError is a convenience method for the cache manager to execute functions with cache read lock that return errors.
func (m *SCIPCacheManager) WithCacheReadLockError(fn func() (interface{}, error)) (interface{}, error) {
	return WithReadLockError(&m.mu, fn)
}

// WithCacheWriteLockError is a convenience method for the cache manager to execute functions with cache write lock that return errors.
func (m *SCIPCacheManager) WithCacheWriteLockError(fn func() (interface{}, error)) (interface{}, error) {
	return WithWriteLockError(&m.mu, fn)
}

// WithCacheWriteLockVoid is a convenience method for the cache manager to execute functions with cache write lock without return values.
func (m *SCIPCacheManager) WithCacheWriteLockVoid(fn func()) {
	WithWriteLockVoid(&m.mu, fn)
}

// SafeCounter provides a thread-safe counter utility for progress tracking
type SafeCounter struct {
	mu    sync.Mutex
	value int
}

// NewSafeCounter creates a new thread-safe counter
func NewSafeCounter() *SafeCounter {
	return &SafeCounter{}
}

// Increment safely increments the counter and returns the new value
func (c *SafeCounter) Increment() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
	return c.value
}

// Add safely adds a value to the counter and returns the new value
func (c *SafeCounter) Add(delta int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += delta
	return c.value
}

// Get safely retrieves the current counter value
func (c *SafeCounter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// Reset safely resets the counter to zero
func (c *SafeCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = 0
}
