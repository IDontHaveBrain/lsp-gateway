package storage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Storage configuration validation constants
const (
	// Capacity parsing
	MinCapacityBytes = 1024 * 1024      // 1MB minimum
	MaxCapacityBytes = 1024 * 1024 * 1024 * 1024 * 10 // 10TB maximum
	
	// Entry limits
	MinMaxEntries = 100
	MaxMaxEntries = 100 * 1000 * 1000 // 100M entries
	
	// Performance limits
	MinConcurrency = 1
	MaxConcurrency = 1000
	MinTimeoutMs = 100
	MaxTimeoutMs = 300000 // 5 minutes
	MinBatchSize = 1
	MaxBatchSize = 10000
	
	// Reliability limits
	MinRetryCount = 0
	MaxRetryCount = 10
	MinRetryDelayMs = 10
	MaxRetryDelayMs = 60000 // 1 minute
	MinFailureThreshold = 1
	MaxFailureThreshold = 100
	
	// Access tracking limits
	MinSampleSize = 5
	MaxSampleSize = 10000
	MinConfidenceThreshold = 0.1
	MaxConfidenceThreshold = 1.0
	MinMaxTrackedKeys = 100
	MaxMaxTrackedKeys = 10 * 1000 * 1000 // 10M keys
)

// Valid configuration values
var (
	ValidBackendTypes = []string{
		"memory", "local_disk", "network_disk", "s3", "redis", "database", "custom",
	}
	
	ValidCompressionAlgorithms = []string{
		"none", "gzip", "lz4", "zstd", "snappy",
	}
	
	ValidEncryptionAlgorithms = []string{
		"none", "aes256", "chacha20", "aes-gcm",
	}
	
	ValidEvictionPolicies = []string{
		"lru", "lfu", "ttl", "size", "adaptive", "predictive", "custom",
	}
	
	ValidPromotionStrategies = []string{
		"lfu", "lru", "adaptive", "predictive", "hybrid", "custom",
	}
	
	ValidAuthenticationTypes = []string{
		"none", "basic", "bearer", "aws_iam", "oauth2", "client_cert",
	}
	
	ValidLogLevels = []string{
		"debug", "info", "warn", "error", "fatal",
	}
	
	ValidExportFormats = []string{
		"json", "prometheus", "csv", "influxdb",
	}
	
	ValidClassificationLevels = []string{
		"public", "internal", "confidential", "restricted",
	}
)

// StorageConfigValidator provides validation for storage configuration
type StorageConfigValidator struct {
	errors []string
}

// NewStorageConfigValidator creates a new storage configuration validator
func NewStorageConfigValidator() *StorageConfigValidator {
	return &StorageConfigValidator{
		errors: make([]string, 0),
	}
}

// ValidateStorageConfiguration validates the complete storage configuration
func ValidateStorageConfiguration(config *StorageConfiguration) error {
	if config == nil {
		return fmt.Errorf("storage configuration cannot be nil")
	}
	
	validator := NewStorageConfigValidator()
	
	// Validate basic configuration
	validator.validateBasicConfig(config)
	
	// Validate tiers
	if config.Tiers != nil {
		validator.validateTiersConfig(config.Tiers)
	}
	
	// Validate strategy
	if config.Strategy != nil {
		validator.validateStrategyConfig(config.Strategy)
	}
	
	// Validate monitoring
	if config.Monitoring != nil {
		validator.validateMonitoringConfig(config.Monitoring)
	}
	
	// Validate maintenance
	if config.Maintenance != nil {
		validator.validateMaintenanceConfig(config.Maintenance)
	}
	
	// Validate security
	if config.Security != nil {
		validator.validateSecurityConfig(config.Security)
	}
	
	// Cross-validation
	validator.validateCrossConfiguration(config)
	
	return validator.getError()
}

// validateBasicConfig validates basic storage configuration
func (v *StorageConfigValidator) validateBasicConfig(config *StorageConfiguration) {
	if config.Version == "" {
		v.addError("storage version cannot be empty")
	}
	
	if config.Profile != "" {
		validProfiles := []string{"development", "production", "analysis", "custom"}
		if !contains(validProfiles, config.Profile) {
			v.addError("invalid storage profile: %s, must be one of %v", config.Profile, validProfiles)
		}
	}
}

// validateTiersConfig validates storage tiers configuration
func (v *StorageConfigValidator) validateTiersConfig(config *StorageTiersConfig) {
	enabledTiers := 0
	
	if config.L1Memory != nil {
		v.validateTierConfiguration("L1Memory", config.L1Memory)
		if config.L1Memory.Enabled {
			enabledTiers++
		}
	}
	
	if config.L2Disk != nil {
		v.validateTierConfiguration("L2Disk", config.L2Disk)
		if config.L2Disk.Enabled {
			enabledTiers++
		}
	}
	
	if config.L3Remote != nil {
		v.validateTierConfiguration("L3Remote", config.L3Remote)
		if config.L3Remote.Enabled {
			enabledTiers++
		}
	}
	
	if enabledTiers == 0 {
		v.addError("at least one storage tier must be enabled")
	}
}

// validateTierConfiguration validates individual tier configuration
func (v *StorageConfigValidator) validateTierConfiguration(tierName string, config *TierConfiguration) {
	prefix := fmt.Sprintf("tier %s", tierName)
	
	// Validate capacity
	if config.Capacity != "" && config.Capacity != "unlimited" {
		if _, err := ParseCapacity(config.Capacity); err != nil {
			v.addError("%s: invalid capacity format: %s", prefix, err.Error())
		}
	}
	
	// Validate max entries
	if config.MaxEntries != -1 && (config.MaxEntries < MinMaxEntries || config.MaxEntries > MaxMaxEntries) {
		v.addError("%s: max entries must be between %d and %d or -1 for unlimited", prefix, MinMaxEntries, MaxMaxEntries)
	}
	
	// Validate eviction policy
	if config.EvictionPolicy != "" && !contains(ValidEvictionPolicies, config.EvictionPolicy) {
		v.addError("%s: invalid eviction policy: %s, must be one of %v", prefix, config.EvictionPolicy, ValidEvictionPolicies)
	}
	
	// Validate backend
	if config.Backend != nil {
		v.validateBackendConfiguration(prefix, config.Backend)
	}
	
	// Validate compression
	if config.Compression != nil {
		v.validateCompressionConfiguration(prefix, config.Compression)
	}
	
	// Validate encryption
	if config.Encryption != nil {
		v.validateEncryptionConfiguration(prefix, config.Encryption)
	}
	
	// Validate performance
	if config.Performance != nil {
		v.validateTierPerformanceConfig(prefix, config.Performance)
	}
	
	// Validate reliability
	if config.Reliability != nil {
		v.validateTierReliabilityConfig(prefix, config.Reliability)
	}
}

// validateBackendConfiguration validates backend configuration
func (v *StorageConfigValidator) validateBackendConfiguration(prefix string, config *BackendConfiguration) {
	if config.Type == "" {
		v.addError("%s: backend type cannot be empty", prefix)
		return
	}
	
	if !contains(ValidBackendTypes, config.Type) {
		v.addError("%s: invalid backend type: %s, must be one of %v", prefix, config.Type, ValidBackendTypes)
	}
	
	// Type-specific validations
	switch config.Type {
	case "local_disk":
		if config.Path == "" {
			v.addError("%s: path is required for local_disk backend", prefix)
		}
	case "s3":
		if config.Bucket == "" {
			v.addError("%s: bucket is required for s3 backend", prefix)
		}
		if config.Region == "" {
			v.addError("%s: region is required for s3 backend", prefix)
		}
	case "redis":
		if config.Endpoint == "" {
			v.addError("%s: endpoint is required for redis backend", prefix)
		}
	case "database":
		if config.ConnectionString == "" {
			v.addError("%s: connection_string is required for database backend", prefix)
		}
	}
	
	// Validate authentication
	if config.Authentication != nil {
		v.validateAuthenticationConfig(prefix, config.Authentication)
	}
}

// validateCompressionConfiguration validates compression configuration
func (v *StorageConfigValidator) validateCompressionConfiguration(prefix string, config *CompressionConfiguration) {
	if config.Enabled {
		if config.Algorithm != "" && !contains(ValidCompressionAlgorithms, config.Algorithm) {
			v.addError("%s: invalid compression algorithm: %s, must be one of %v", prefix, config.Algorithm, ValidCompressionAlgorithms)
		}
		
		if config.Level < 0 || config.Level > 9 {
			v.addError("%s: compression level must be between 0 and 9", prefix)
		}
		
		if config.MinSize < 0 {
			v.addError("%s: compression min_size must be non-negative", prefix)
		}
		
		if config.Threshold < 0 || config.Threshold > 1 {
			v.addError("%s: compression threshold must be between 0 and 1", prefix)
		}
	}
}

// validateEncryptionConfiguration validates encryption configuration
func (v *StorageConfigValidator) validateEncryptionConfiguration(prefix string, config *EncryptionConfiguration) {
	if config.Enabled {
		if config.Algorithm != "" && !contains(ValidEncryptionAlgorithms, config.Algorithm) {
			v.addError("%s: invalid encryption algorithm: %s, must be one of %v", prefix, config.Algorithm, ValidEncryptionAlgorithms)
		}
		
		if config.KeySize != 0 {
			validKeySizes := []int{128, 192, 256}
			if !containsInt(validKeySizes, config.KeySize) {
				v.addError("%s: invalid key size: %d, must be one of %v", prefix, config.KeySize, validKeySizes)
			}
		}
		
		if config.KeyRotation < 0 {
			v.addError("%s: key rotation duration must be non-negative", prefix)
		}
	}
}

// validateTierPerformanceConfig validates tier performance configuration
func (v *StorageConfigValidator) validateTierPerformanceConfig(prefix string, config *TierPerformanceConfig) {
	if config.MaxConcurrency < MinConcurrency || config.MaxConcurrency > MaxConcurrency {
		v.addError("%s: max_concurrency must be between %d and %d", prefix, MinConcurrency, MaxConcurrency)
	}
	
	if config.TimeoutMs < MinTimeoutMs || config.TimeoutMs > MaxTimeoutMs {
		v.addError("%s: timeout_ms must be between %d and %d", prefix, MinTimeoutMs, MaxTimeoutMs)
	}
	
	if config.BatchSize < MinBatchSize || config.BatchSize > MaxBatchSize {
		v.addError("%s: batch_size must be between %d and %d", prefix, MinBatchSize, MaxBatchSize)
	}
	
	if config.ReadBufferSize < 0 {
		v.addError("%s: read_buffer_size must be non-negative", prefix)
	}
	
	if config.WriteBufferSize < 0 {
		v.addError("%s: write_buffer_size must be non-negative", prefix)
	}
	
	if config.ConnectionPoolSize < 0 {
		v.addError("%s: connection_pool_size must be non-negative", prefix)
	}
	
	if config.PrefetchSize < 0 {
		v.addError("%s: prefetch_size must be non-negative", prefix)
	}
}

// validateTierReliabilityConfig validates tier reliability configuration
func (v *StorageConfigValidator) validateTierReliabilityConfig(prefix string, config *TierReliabilityConfig) {
	if config.RetryCount < MinRetryCount || config.RetryCount > MaxRetryCount {
		v.addError("%s: retry_count must be between %d and %d", prefix, MinRetryCount, MaxRetryCount)
	}
	
	if config.RetryDelayMs < MinRetryDelayMs || config.RetryDelayMs > MaxRetryDelayMs {
		v.addError("%s: retry_delay_ms must be between %d and %d", prefix, MinRetryDelayMs, MaxRetryDelayMs)
	}
	
	if config.FailureThreshold < MinFailureThreshold || config.FailureThreshold > MaxFailureThreshold {
		v.addError("%s: failure_threshold must be between %d and %d", prefix, MinFailureThreshold, MaxFailureThreshold)
	}
	
	if config.HealthCheckInterval < 0 {
		v.addError("%s: health_check_interval must be non-negative", prefix)
	}
	
	if config.RecoveryTimeout < 0 {
		v.addError("%s: recovery_timeout must be non-negative", prefix)
	}
	
	if config.ReplicationFactor < 0 {
		v.addError("%s: replication_factor must be non-negative", prefix)
	}
	
	// Validate circuit breaker
	if config.CircuitBreaker != nil {
		v.validateCircuitBreakerConfiguration(prefix, config.CircuitBreaker)
	}
}

// validateCircuitBreakerConfiguration validates circuit breaker configuration
func (v *StorageConfigValidator) validateCircuitBreakerConfiguration(prefix string, config *CircuitBreakerConfiguration) {
	if config.Enabled {
		if config.FailureThreshold < 1 {
			v.addError("%s: circuit breaker failure_threshold must be at least 1", prefix)
		}
		
		if config.RecoveryTimeout < 0 {
			v.addError("%s: circuit breaker recovery_timeout must be non-negative", prefix)
		}
		
		if config.HalfOpenRequests < 1 {
			v.addError("%s: circuit breaker half_open_requests must be at least 1", prefix)
		}
		
		if config.MinRequestCount < 1 {
			v.addError("%s: circuit breaker min_request_count must be at least 1", prefix)
		}
	}
}

// validateAuthenticationConfig validates authentication configuration
func (v *StorageConfigValidator) validateAuthenticationConfig(prefix string, config *AuthenticationConfig) {
	if config.Type != "" && !contains(ValidAuthenticationTypes, config.Type) {
		v.addError("%s: invalid authentication type: %s, must be one of %v", prefix, config.Type, ValidAuthenticationTypes)
	}
	
	switch config.Type {
	case "basic":
		if config.Username == "" {
			v.addError("%s: username is required for basic authentication", prefix)
		}
		if config.Password == "" {
			v.addError("%s: password is required for basic authentication", prefix)
		}
	case "bearer":
		if config.Token == "" {
			v.addError("%s: token is required for bearer authentication", prefix)
		}
	case "client_cert":
		if config.CertPath == "" {
			v.addError("%s: cert_path is required for client certificate authentication", prefix)
		}
		if config.KeyPath == "" {
			v.addError("%s: key_path is required for client certificate authentication", prefix)
		}
	}
}

// validateStrategyConfig validates strategy configuration
func (v *StorageConfigValidator) validateStrategyConfig(config *StorageStrategyConfig) {
	if config.PromotionStrategy != nil {
		v.validatePromotionStrategyConfig(config.PromotionStrategy)
	}
	
	if config.EvictionPolicy != nil {
		v.validateEvictionPolicyConfig(config.EvictionPolicy)
	}
	
	if config.AccessTracking != nil {
		v.validateAccessTrackingConfig(config.AccessTracking)
	}
	
	if config.AutoOptimization != nil {
		v.validateAutoOptimizationConfig(config.AutoOptimization)
	}
}

// validatePromotionStrategyConfig validates promotion strategy configuration
func (v *StorageConfigValidator) validatePromotionStrategyConfig(config *PromotionStrategyConfig) {
	if config.Type != "" && !contains(ValidPromotionStrategies, config.Type) {
		v.addError("invalid promotion strategy type: %s, must be one of %v", config.Type, ValidPromotionStrategies)
	}
	
	if config.MinAccessCount < 0 {
		v.addError("promotion strategy min_access_count must be non-negative")
	}
	
	if config.MinAccessFrequency < 0 || config.MinAccessFrequency > 1 {
		v.addError("promotion strategy min_access_frequency must be between 0 and 1")
	}
	
	if config.AccessTimeWindow < 0 {
		v.addError("promotion strategy access_time_window must be non-negative")
	}
	
	if config.PromotionCooldown < 0 {
		v.addError("promotion strategy promotion_cooldown must be non-negative")
	}
	
	// Validate weights
	if config.RecencyWeight < 0 || config.RecencyWeight > 1 {
		v.addError("promotion strategy recency_weight must be between 0 and 1")
	}
	
	if config.FrequencyWeight < 0 || config.FrequencyWeight > 1 {
		v.addError("promotion strategy frequency_weight must be between 0 and 1")
	}
	
	if config.SizeWeight < 0 || config.SizeWeight > 1 {
		v.addError("promotion strategy size_weight must be between 0 and 1")
	}
	
	// Validate tier thresholds
	for tier, threshold := range config.TierThresholds {
		if threshold < 0 || threshold > 1 {
			v.addError("promotion strategy tier threshold for %s must be between 0 and 1", tier)
		}
	}
}

// validateEvictionPolicyConfig validates eviction policy configuration
func (v *StorageConfigValidator) validateEvictionPolicyConfig(config *EvictionPolicyConfig) {
	if config.Type != "" && !contains(ValidEvictionPolicies, config.Type) {
		v.addError("invalid eviction policy type: %s, must be one of %v", config.Type, ValidEvictionPolicies)
	}
	
	if config.EvictionThreshold < 0 || config.EvictionThreshold > 1 {
		v.addError("eviction policy eviction_threshold must be between 0 and 1")
	}
	
	if config.TargetUtilization < 0 || config.TargetUtilization > 1 {
		v.addError("eviction policy target_utilization must be between 0 and 1")
	}
	
	if config.EvictionBatchSize < 1 {
		v.addError("eviction policy eviction_batch_size must be at least 1")
	}
	
	if config.AccessAgeThreshold < 0 {
		v.addError("eviction policy access_age_threshold must be non-negative")
	}
	
	if config.InactivityThreshold < 0 {
		v.addError("eviction policy inactivity_threshold must be non-negative")
	}
	
	if config.DefaultTTL < 0 {
		v.addError("eviction policy default_ttl must be non-negative")
	}
	
	if config.MaxTTL < 0 {
		v.addError("eviction policy max_ttl must be non-negative")
	}
	
	if config.SizeWeight < 0 || config.SizeWeight > 1 {
		v.addError("eviction policy size_weight must be between 0 and 1")
	}
}

// validateAccessTrackingConfig validates access tracking configuration
func (v *StorageConfigValidator) validateAccessTrackingConfig(config *AccessTrackingConfig) {
	if config.TrackingGranularity < 0 {
		v.addError("access tracking tracking_granularity must be non-negative")
	}
	
	if config.HistoryRetention < 0 {
		v.addError("access tracking history_retention must be non-negative")
	}
	
	if config.MaxTrackedKeys < MinMaxTrackedKeys || config.MaxTrackedKeys > MaxMaxTrackedKeys {
		v.addError("access tracking max_tracked_keys must be between %d and %d", MinMaxTrackedKeys, MaxMaxTrackedKeys)
	}
	
	if config.AnalysisInterval < 0 {
		v.addError("access tracking analysis_interval must be non-negative")
	}
	
	if config.MinSampleSize < MinSampleSize || config.MinSampleSize > MaxSampleSize {
		v.addError("access tracking min_sample_size must be between %d and %d", MinSampleSize, MaxSampleSize)
	}
	
	if config.ConfidenceThreshold < MinConfidenceThreshold || config.ConfidenceThreshold > MaxConfidenceThreshold {
		v.addError("access tracking confidence_threshold must be between %f and %f", MinConfidenceThreshold, MaxConfidenceThreshold)
	}
}

// validateAutoOptimizationConfig validates auto optimization configuration
func (v *StorageConfigValidator) validateAutoOptimizationConfig(config *AutoOptimizationConfig) {
	if config.OptimizationInterval < 0 {
		v.addError("auto optimization optimization_interval must be non-negative")
	}
	
	if config.PerformanceThreshold < 0 || config.PerformanceThreshold > 1 {
		v.addError("auto optimization performance_threshold must be between 0 and 1")
	}
	
	if config.CapacityThreshold < 0 || config.CapacityThreshold > 1 {
		v.addError("auto optimization capacity_threshold must be between 0 and 1")
	}
}

// validateMonitoringConfig validates monitoring configuration
func (v *StorageConfigValidator) validateMonitoringConfig(config *StorageMonitoringConfig) {
	if config.MetricsInterval < 0 {
		v.addError("monitoring metrics_interval must be non-negative")
	}
	
	if config.HealthInterval < 0 {
		v.addError("monitoring health_interval must be non-negative")
	}
	
	if config.LogLevel != "" && !contains(ValidLogLevels, config.LogLevel) {
		v.addError("invalid monitoring log_level: %s, must be one of %v", config.LogLevel, ValidLogLevels)
	}
	
	if config.ExportFormat != "" && !contains(ValidExportFormats, config.ExportFormat) {
		v.addError("invalid monitoring export_format: %s, must be one of %v", config.ExportFormat, ValidExportFormats)
	}
	
	if config.RetentionPeriod < 0 {
		v.addError("monitoring retention_period must be non-negative")
	}
	
	// Validate alert thresholds
	for metric, threshold := range config.AlertThresholds {
		switch metric {
		case "hit_rate_low", "error_rate_high", "capacity_high":
			if threshold < 0 || threshold > 1 {
				v.addError("monitoring alert threshold for %s must be between 0 and 1", metric)
			}
		case "latency_high":
			if threshold < 0 {
				v.addError("monitoring alert threshold for %s must be non-negative", metric)
			}
		}
	}
}

// validateMaintenanceConfig validates maintenance configuration
func (v *StorageConfigValidator) validateMaintenanceConfig(config *StorageMaintenanceConfig) {
	if config.Schedule != "" {
		if err := ValidateCronExpression(config.Schedule); err != nil {
			v.addError("invalid maintenance schedule: %s", err.Error())
		}
	}
	
	if config.CompactionThreshold < 0 || config.CompactionThreshold > 1 {
		v.addError("maintenance compaction_threshold must be between 0 and 1")
	}
	
	if config.VacuumInterval < 0 {
		v.addError("maintenance vacuum_interval must be non-negative")
	}
	
	if config.CleanupAge < 0 {
		v.addError("maintenance cleanup_age must be non-negative")
	}
	
	if config.BackupInterval < 0 {
		v.addError("maintenance backup_interval must be non-negative")
	}
	
	if config.BackupRetention < 0 {
		v.addError("maintenance backup_retention must be non-negative")
	}
	
	// Validate maintenance window
	if config.MaintenanceWindow != nil {
		v.validateMaintenanceWindowConfig(config.MaintenanceWindow)
	}
}

// validateMaintenanceWindowConfig validates maintenance window configuration
func (v *StorageConfigValidator) validateMaintenanceWindowConfig(config *MaintenanceWindowConfig) {
	if config.StartTime != "" {
		if !isValidTimeFormat(config.StartTime) {
			v.addError("invalid maintenance window start_time format, expected HH:MM")
		}
	}
	
	if config.EndTime != "" {
		if !isValidTimeFormat(config.EndTime) {
			v.addError("invalid maintenance window end_time format, expected HH:MM")
		}
	}
	
	validDays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for _, day := range config.Days {
		if !contains(validDays, strings.ToLower(day)) {
			v.addError("invalid maintenance window day: %s, must be one of %v", day, validDays)
		}
	}
}

// validateSecurityConfig validates security configuration
func (v *StorageConfigValidator) validateSecurityConfig(config *StorageSecurityConfig) {
	if config.AccessControl != nil {
		v.validateAccessControlConfig(config.AccessControl)
	}
	
	if config.AuditLogging != nil {
		v.validateAuditLoggingConfig(config.AuditLogging)
	}
	
	if config.DataClassification != nil {
		v.validateDataClassificationConfig(config.DataClassification)
	}
}

// validateAccessControlConfig validates access control configuration
func (v *StorageConfigValidator) validateAccessControlConfig(config *AccessControlConfig) {
	if config.DefaultPolicy != "" {
		validPolicies := []string{"allow", "deny"}
		if !contains(validPolicies, config.DefaultPolicy) {
			v.addError("invalid access control default_policy: %s, must be one of %v", config.DefaultPolicy, validPolicies)
		}
	}
	
	if config.RateLimiting != nil {
		v.validateRateLimitingConfig(config.RateLimiting)
	}
}

// validateRateLimitingConfig validates rate limiting configuration
func (v *StorageConfigValidator) validateRateLimitingConfig(config *RateLimitingConfig) {
	if config.RequestsPerMinute < 0 {
		v.addError("rate limiting requests_per_minute must be non-negative")
	}
	
	if config.BurstSize < 0 {
		v.addError("rate limiting burst_size must be non-negative")
	}
	
	if config.WindowSize < 0 {
		v.addError("rate limiting window_size must be non-negative")
	}
}

// validateAuditLoggingConfig validates audit logging configuration
func (v *StorageConfigValidator) validateAuditLoggingConfig(config *AuditLoggingConfig) {
	if config.LogLevel != "" && !contains(ValidLogLevels, config.LogLevel) {
		v.addError("invalid audit logging log_level: %s, must be one of %v", config.LogLevel, ValidLogLevels)
	}
	
	if config.RetentionDays < 0 {
		v.addError("audit logging retention_days must be non-negative")
	}
	
	if config.LogFormat != "" {
		validFormats := []string{"json", "text", "csv"}
		if !contains(validFormats, config.LogFormat) {
			v.addError("invalid audit logging log_format: %s, must be one of %v", config.LogFormat, validFormats)
		}
	}
}

// validateDataClassificationConfig validates data classification configuration
func (v *StorageConfigValidator) validateDataClassificationConfig(config *DataClassificationConfig) {
	if config.DefaultLevel != "" && !contains(ValidClassificationLevels, config.DefaultLevel) {
		v.addError("invalid data classification default_level: %s, must be one of %v", config.DefaultLevel, ValidClassificationLevels)
	}
	
	for _, level := range config.ClassificationRules {
		if !contains(ValidClassificationLevels, level) {
			v.addError("invalid data classification level in rules: %s, must be one of %v", level, ValidClassificationLevels)
		}
	}
}

// validateCrossConfiguration performs cross-validation across configuration sections
func (v *StorageConfigValidator) validateCrossConfiguration(config *StorageConfiguration) {
	// Validate that enabled features have proper dependencies
	if config.Strategy != nil && config.Strategy.AccessTracking != nil && config.Strategy.AccessTracking.Enabled {
		if config.Monitoring == nil || !config.Monitoring.Enabled {
			v.addError("access tracking requires monitoring to be enabled")
		}
	}
	
	if config.Strategy != nil && config.Strategy.AutoOptimization != nil && config.Strategy.AutoOptimization.Enabled {
		if config.Strategy.AccessTracking == nil || !config.Strategy.AccessTracking.Enabled {
			v.addError("auto optimization requires access tracking to be enabled")
		}
	}
	
	// Validate tier hierarchy
	if config.Tiers != nil {
		l1Enabled := config.Tiers.L1Memory != nil && config.Tiers.L1Memory.Enabled
		l2Enabled := config.Tiers.L2Disk != nil && config.Tiers.L2Disk.Enabled
		l3Enabled := config.Tiers.L3Remote != nil && config.Tiers.L3Remote.Enabled
		
		if l3Enabled && !l2Enabled && !l1Enabled {
			v.addError("L3 remote tier cannot be enabled without L1 or L2 tiers")
		}
		
		if l2Enabled && !l1Enabled {
			v.addWarning("L2 disk tier is enabled without L1 memory tier, which may impact performance")
		}
	}
	
	// Validate security consistency
	if config.Security != nil {
		if config.Security.EncryptionAtRest && config.Tiers != nil {
			// Check that encryption is configured for tiers that support it
			if config.Tiers.L2Disk != nil && config.Tiers.L2Disk.Enabled {
				if config.Tiers.L2Disk.Encryption == nil || !config.Tiers.L2Disk.Encryption.Enabled {
					v.addWarning("encryption at rest is enabled but L2 disk tier does not have encryption configured")
				}
			}
			
			if config.Tiers.L3Remote != nil && config.Tiers.L3Remote.Enabled {
				if config.Tiers.L3Remote.Encryption == nil || !config.Tiers.L3Remote.Encryption.Enabled {
					v.addWarning("encryption at rest is enabled but L3 remote tier does not have encryption configured")
				}
			}
		}
	}
}

// Helper functions

// addError adds an error to the validator
func (v *StorageConfigValidator) addError(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

// addWarning adds a warning to the validator (currently just treated as error for simplicity)
func (v *StorageConfigValidator) addWarning(format string, args ...interface{}) {
	// For now, treat warnings as errors for strict validation
	// In the future, we could separate warnings from errors
	v.addError("warning: "+format, args...)
}

// getError returns the accumulated validation errors
func (v *StorageConfigValidator) getError() error {
	if len(v.errors) == 0 {
		return nil
	}
	
	return fmt.Errorf("storage configuration validation failed:\n- %s", strings.Join(v.errors, "\n- "))
}

// Utility functions

// ParseCapacity parses a capacity string (e.g., "4GB", "200MB") into bytes
func ParseCapacity(capacity string) (int64, error) {
	if capacity == "" || capacity == "unlimited" {
		return -1, nil
	}
	
	// Regular expression to parse capacity string
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]?B?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(capacity))
	
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid capacity format: %s, expected format like '4GB', '200MB'", capacity)
	}
	
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value in capacity: %s", matches[1])
	}
	
	unit := matches[2]
	var multiplier int64
	
	switch unit {
	case "", "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid capacity unit: %s, supported units: B, KB, MB, GB, TB", unit)
	}
	
	bytes := int64(value * float64(multiplier))
	
	if bytes < MinCapacityBytes {
		return 0, fmt.Errorf("capacity too small: %d bytes, minimum: %d bytes", bytes, MinCapacityBytes)
	}
	
	if bytes > MaxCapacityBytes {
		return 0, fmt.Errorf("capacity too large: %d bytes, maximum: %d bytes", bytes, MaxCapacityBytes)
	}
	
	return bytes, nil
}

// ValidateCronExpression validates a cron expression
func ValidateCronExpression(expr string) error {
	// Basic cron expression validation (5 or 6 fields)
	fields := strings.Fields(expr)
	if len(fields) != 5 && len(fields) != 6 {
		return fmt.Errorf("cron expression must have 5 or 6 fields, got %d", len(fields))
	}
	
	// Basic field validation (simplified)
	fieldNames := []string{"minute", "hour", "day", "month", "weekday"}
	if len(fields) == 6 {
		fieldNames = append([]string{"second"}, fieldNames...)
	}
	
	for i, field := range fields {
		if field == "*" || field == "?" {
			continue
		}
		
		// Check for ranges, lists, and steps
		if strings.Contains(field, "-") || strings.Contains(field, ",") || strings.Contains(field, "/") {
			continue
		}
		
		// Check if it's a valid number for the field
		num, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("invalid value for %s field: %s", fieldNames[i], field)
		}
		
		// Basic range validation
		switch fieldNames[i] {
		case "second", "minute":
			if num < 0 || num > 59 {
				return fmt.Errorf("%s must be between 0 and 59, got %d", fieldNames[i], num)
			}
		case "hour":
			if num < 0 || num > 23 {
				return fmt.Errorf("hour must be between 0 and 23, got %d", num)
			}
		case "day":
			if num < 1 || num > 31 {
				return fmt.Errorf("day must be between 1 and 31, got %d", num)
			}
		case "month":
			if num < 1 || num > 12 {
				return fmt.Errorf("month must be between 1 and 12, got %d", num)
			}
		case "weekday":
			if num < 0 || num > 7 {
				return fmt.Errorf("weekday must be between 0 and 7, got %d", num)
			}
		}
	}
	
	return nil
}

// isValidTimeFormat validates time format (HH:MM)
func isValidTimeFormat(timeStr string) bool {
	re := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]$`)
	return re.MatchString(timeStr)
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsInt checks if an int slice contains a specific int
func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Configuration helpers for environment variable overrides

// ApplyStorageEnvironmentVariables applies environment variable overrides to storage configuration
func ApplyStorageEnvironmentVariables(config *StorageConfiguration) error {
	// This would contain logic to apply environment variable overrides
	// Similar to the existing ApplySCIPEnvironmentVariables function
	// For now, this is a placeholder for future implementation
	return nil
}

// GetDefaultStorageConfigForProfile returns default storage configuration for a specific profile
func GetDefaultStorageConfigForProfile(profile string) *StorageConfiguration {
	defaultConfig := DefaultStorageConfiguration()
	
	switch profile {
	case "production":
		return getProductionStorageConfig(defaultConfig)
	case "development":
		return getDevelopmentStorageConfig(defaultConfig)
	case "analysis":
		return getAnalysisStorageConfig(defaultConfig)
	default:
		return defaultConfig
	}
}

// getProductionStorageConfig returns production-optimized storage configuration
func getProductionStorageConfig(base *StorageConfiguration) *StorageConfiguration {
	config := *base
	config.Profile = "production"
	config.Enabled = true
	
	// Enable all tiers for production
	if config.Tiers != nil {
		if config.Tiers.L1Memory != nil {
			config.Tiers.L1Memory.Enabled = true
			config.Tiers.L1Memory.Capacity = "8GB"
		}
		if config.Tiers.L2Disk != nil {
			config.Tiers.L2Disk.Enabled = true
			config.Tiers.L2Disk.Capacity = "500GB"
		}
		if config.Tiers.L3Remote != nil {
			config.Tiers.L3Remote.Enabled = true
		}
	}
	
	// Enable monitoring and security
	if config.Monitoring != nil {
		config.Monitoring.Enabled = true
		config.Monitoring.DetailedMetrics = true
	}
	
	if config.Security != nil {
		config.Security.EncryptionAtRest = true
		config.Security.EncryptionInTransit = true
		if config.Security.AuditLogging != nil {
			config.Security.AuditLogging.Enabled = true
		}
	}
	
	// Enable auto-optimization
	if config.Strategy != nil && config.Strategy.AutoOptimization != nil {
		config.Strategy.AutoOptimization.Enabled = true
	}
	
	return &config
}

// getDevelopmentStorageConfig returns development-optimized storage configuration
func getDevelopmentStorageConfig(base *StorageConfiguration) *StorageConfiguration {
	config := *base
	config.Profile = "development"
	config.Enabled = false // Disabled by default for development
	
	// Only enable L1 memory tier for development
	if config.Tiers != nil {
		if config.Tiers.L1Memory != nil {
			config.Tiers.L1Memory.Enabled = true
			config.Tiers.L1Memory.Capacity = "2GB"
		}
		if config.Tiers.L2Disk != nil {
			config.Tiers.L2Disk.Enabled = false
		}
		if config.Tiers.L3Remote != nil {
			config.Tiers.L3Remote.Enabled = false
		}
	}
	
	// Simplified monitoring for development
	if config.Monitoring != nil {
		config.Monitoring.Enabled = true
		config.Monitoring.DetailedMetrics = false
		config.Monitoring.LogLevel = "debug"
	}
	
	// Minimal security for development
	if config.Security != nil {
		config.Security.EncryptionAtRest = false
		config.Security.EncryptionInTransit = false
	}
	
	return &config
}

// getAnalysisStorageConfig returns analysis-optimized storage configuration
func getAnalysisStorageConfig(base *StorageConfiguration) *StorageConfiguration {
	config := *base
	config.Profile = "analysis"
	config.Enabled = true
	
	// Enable all tiers with large capacities for analysis
	if config.Tiers != nil {
		if config.Tiers.L1Memory != nil {
			config.Tiers.L1Memory.Enabled = true
			config.Tiers.L1Memory.Capacity = "16GB"
		}
		if config.Tiers.L2Disk != nil {
			config.Tiers.L2Disk.Enabled = true
			config.Tiers.L2Disk.Capacity = "1TB"
		}
		if config.Tiers.L3Remote != nil {
			config.Tiers.L3Remote.Enabled = true
		}
	}
	
	// Enhanced monitoring and access tracking for analysis
	if config.Monitoring != nil {
		config.Monitoring.Enabled = true
		config.Monitoring.DetailedMetrics = true
		config.Monitoring.TraceRequests = true
	}
	
	if config.Strategy != nil && config.Strategy.AccessTracking != nil {
		config.Strategy.AccessTracking.Enabled = true
		config.Strategy.AccessTracking.SemanticAnalysis = true
		config.Strategy.AccessTracking.SeasonalityDetection = true
		config.Strategy.AccessTracking.TrendDetection = true
	}
	
	return &config
}