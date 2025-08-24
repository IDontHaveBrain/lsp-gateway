package common

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// CommandContext encapsulates common CLI command lifecycle components
type CommandContext struct {
	Config  *config.Config
	Manager *server.LSPManager
	Cache   cache.SCIPCache
	Context context.Context
	Cancel  context.CancelFunc

	// Internal state
	managerStarted bool
	cacheOnly      bool
}

// CommandContextOptions configures CommandContext creation
type CommandContextOptions struct {
	Timeout   time.Duration
	CacheOnly bool // Use cache-only mode (no LSP servers)
	SkipStart bool // Don't start manager automatically
	SkipCache bool // Don't initialize cache
}

// NewCommandContext creates a new CommandContext with standard initialization
func NewCommandContext(configPath string, timeout time.Duration) (*CommandContext, error) {
	return NewCommandContextWithOptions(configPath, CommandContextOptions{
		Timeout: timeout,
	})
}

// NewCommandContextWithOptions creates a CommandContext with custom options
func NewCommandContextWithOptions(configPath string, opts CommandContextOptions) (*CommandContext, error) {
	// Load configuration
	cfg := LoadConfigForCLI(configPath)

	// Create context with timeout
	ctx, cancel := common.CreateContext(opts.Timeout)

	cmdCtx := &CommandContext{
		Config:    cfg,
		Context:   ctx,
		Cancel:    cancel,
		cacheOnly: opts.CacheOnly,
	}

	// Create cache or manager based on options
	if opts.CacheOnly {
		if err := cmdCtx.initCacheOnly(); err != nil {
			cmdCtx.Cleanup()
			return nil, fmt.Errorf("failed to initialize cache: %w", err)
		}
	} else {
		if err := cmdCtx.initManager(); err != nil {
			cmdCtx.Cleanup()
			return nil, fmt.Errorf("failed to initialize LSP manager: %w", err)
		}

		// Start manager unless explicitly skipped
		if !opts.SkipStart {
			if err := cmdCtx.startManager(); err != nil {
				cmdCtx.Cleanup()
				return nil, fmt.Errorf("failed to start LSP manager: %w", err)
			}
		}

		// Initialize cache unless explicitly skipped
		if !opts.SkipCache {
			cmdCtx.initCache()
		}
	}

	return cmdCtx, nil
}

// NewCacheOnlyContext creates a context for cache-only operations
func NewCacheOnlyContext(configPath string, timeout time.Duration) (*CommandContext, error) {
	return NewCommandContextWithOptions(configPath, CommandContextOptions{
		Timeout:   timeout,
		CacheOnly: true,
	})
}

// NewManagerContext creates a context with LSP manager but doesn't start it
func NewManagerContext(configPath string, timeout time.Duration) (*CommandContext, error) {
	return NewCommandContextWithOptions(configPath, CommandContextOptions{
		Timeout:   timeout,
		SkipStart: true,
		SkipCache: true,
	})
}

// initManager creates the LSP manager
func (c *CommandContext) initManager() error {
	manager, err := CreateLSPManager(c.Config)
	if err != nil {
		return err
	}
	c.Manager = manager
	return nil
}

// initCacheOnly creates cache without LSP manager
func (c *CommandContext) initCacheOnly() error {
	cacheInstance, err := CreateCacheOnly(c.Config)
	if err != nil {
		return err
	}
	c.Cache = cacheInstance
	return nil
}

// startManager starts the LSP manager
func (c *CommandContext) startManager() error {
	if c.Manager == nil {
		return fmt.Errorf("manager not initialized")
	}

	if err := c.Manager.Start(c.Context); err != nil {
		return err
	}
	c.managerStarted = true
	return nil
}

// initCache initializes cache from manager
func (c *CommandContext) initCache() {
	if c.Manager != nil {
		c.Cache = c.Manager.GetCache()
	}
}

// StartManager starts the LSP manager if not already started
func (c *CommandContext) StartManager() error {
	if c.managerStarted {
		return nil
	}
	return c.startManager()
}

// CheckCacheHealth performs standardized cache health check
func (c *CommandContext) CheckCacheHealth() (*cache.CacheMetrics, error) {
	return CheckCacheHealth(c.Cache)
}

// WaitForInitialization waits for LSP servers to initialize
func (c *CommandContext) WaitForInitialization() {
	if c.managerStarted {
		time.Sleep(2 * time.Second)
	}
}

// ExecuteWithManager executes a function with proper manager lifecycle
func (c *CommandContext) ExecuteWithManager(fn func(*CommandContext) error) error {
	if !c.managerStarted {
		if err := c.StartManager(); err != nil {
			return err
		}
	}

	return fn(c)
}

// ExecuteWithCache executes a function that requires cache
func (c *CommandContext) ExecuteWithCache(fn func(*CommandContext) error) error {
	if c.Cache == nil && c.Manager != nil {
		c.initCache()
	}

	if _, err := c.CheckCacheHealth(); err != nil {
		return err
	}

	return fn(c)
}

// Cleanup properly cleans up all resources
func (c *CommandContext) Cleanup() {
	// Cancel context first
	if c.Cancel != nil {
		c.Cancel()
	}

	// Stop manager if it was started
	if c.Manager != nil && c.managerStarted {
		_ = c.Manager.Stop()
	}

	// Stop cache if cache-only mode
	if c.cacheOnly && c.Cache != nil {
		_ = c.Cache.Stop()
	}
}

// GetWorkingDir returns validated working directory
func (c *CommandContext) GetWorkingDir() (string, error) {
	return common.ValidateAndGetWorkingDir("")
}

// IsManagerStarted returns whether the manager has been started
func (c *CommandContext) IsManagerStarted() bool {
	return c.managerStarted
}

// IsCacheOnly returns whether this is a cache-only context
func (c *CommandContext) IsCacheOnly() bool {
	return c.cacheOnly
}
