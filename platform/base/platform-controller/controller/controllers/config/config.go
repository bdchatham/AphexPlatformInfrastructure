// Package config provides centralized configuration management for the platform controller.
// Configuration values can be loaded from environment variables with sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// Config holds all controller configuration values.
type Config struct {
	PlatformNamespace       string
	AllowlistNamespace      string
	DevelopmentMode         bool
	LeaderElectionEnabled   bool
	ProvisioningTimeout     time.Duration
	DependencyWaitTime      time.Duration
	ShutdownTimeout         time.Duration
	HealthThreshold         time.Duration
	MaxConcurrentReconciles int
	StatusRetryCount        int
	BaseDelay               time.Duration
	MaxDelay                time.Duration
	DefaultResourceQuota    ResourceQuotaConfig
	DefaultLimitRange       LimitRangeConfig
}

// ResourceQuotaConfig defines configurable resource quota limits.
type ResourceQuotaConfig struct {
	CPURequests    resource.Quantity
	CPULimits      resource.Quantity
	MemoryRequests resource.Quantity
	MemoryLimits   resource.Quantity
	PVCCount       int64
	PodCount       int64
}

// LimitRangeConfig defines configurable limit range values.
type LimitRangeConfig struct {
	DefaultCPU     resource.Quantity
	DefaultMemory  resource.Quantity
	RequestCPU     resource.Quantity
	RequestMemory  resource.Quantity
	MaxCPU         resource.Quantity
	MaxMemory      resource.Quantity
	MinCPU         resource.Quantity
	MinMemory      resource.Quantity
}

// Manager loads and provides access to controller configuration.
type Manager struct {
	config *Config
	logger logr.Logger
}

// NewManager creates a new configuration manager.
func NewManager(logger logr.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// LoadFromEnvironment loads configuration from environment variables.
// Missing values use sensible defaults and log warnings.
func (m *Manager) LoadFromEnvironment() error {
	m.config = &Config{
		PlatformNamespace:       m.getEnvOrDefault(constants.EnvPlatformNamespace, constants.DefaultPlatformNamespace),
		AllowlistNamespace:      m.getEnvOrDefault(constants.EnvAllowlistNamespace, constants.DefaultAllowlistNamespace),
		DevelopmentMode:         m.getBoolEnvOrDefault(constants.EnvDevelopmentMode, false),
		LeaderElectionEnabled:   m.getBoolEnvOrDefault(constants.EnvLeaderElectionEnable, true),
		ProvisioningTimeout:     constants.DefaultProvisioningTimeout,
		DependencyWaitTime:      constants.DefaultDependencyWaitTime,
		ShutdownTimeout:         constants.DefaultShutdownTimeout,
		HealthThreshold:         constants.DefaultHealthThreshold,
		MaxConcurrentReconciles: constants.DefaultMaxConcurrentReconciles,
		StatusRetryCount:        constants.DefaultStatusRetryCount,
		BaseDelay:               constants.DefaultBaseDelay,
		MaxDelay:                constants.DefaultMaxDelay,
		DefaultResourceQuota:    DefaultResourceQuota(),
		DefaultLimitRange:       DefaultLimitRange(),
	}

	if err := m.validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	m.logConfiguration()
	return nil
}

// GetConfig returns the loaded configuration.
// Returns nil if LoadFromEnvironment has not been called.
func (m *Manager) GetConfig() *Config {
	return m.config
}

// validate checks that the configuration is valid.
func (m *Manager) validate() error {
	if m.config.PlatformNamespace == "" {
		return fmt.Errorf("platform namespace cannot be empty")
	}
	if m.config.AllowlistNamespace == "" {
		return fmt.Errorf("allowlist namespace cannot be empty")
	}
	if m.config.MaxConcurrentReconciles < 1 {
		return fmt.Errorf("max concurrent reconciles must be at least 1")
	}
	if m.config.StatusRetryCount < 1 {
		return fmt.Errorf("status retry count must be at least 1")
	}
	return nil
}

// logConfiguration logs the loaded configuration values.
func (m *Manager) logConfiguration() {
	m.logger.Info("Configuration loaded",
		"platformNamespace", m.config.PlatformNamespace,
		"allowlistNamespace", m.config.AllowlistNamespace,
		"developmentMode", m.config.DevelopmentMode,
		"leaderElectionEnabled", m.config.LeaderElectionEnabled,
		"maxConcurrentReconciles", m.config.MaxConcurrentReconciles,
	)

	if m.config.PlatformNamespace != constants.DefaultPlatformNamespace {
		m.logger.Info("Using non-default platform namespace", "namespace", m.config.PlatformNamespace)
	}
	if m.config.AllowlistNamespace != constants.DefaultAllowlistNamespace {
		m.logger.Info("Using non-default allowlist namespace", "namespace", m.config.AllowlistNamespace)
	}
}

// getEnvOrDefault returns the environment variable value or the default.
func (m *Manager) getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		if defaultValue != "" {
			m.logger.V(1).Info("Using default value for configuration",
				"key", key,
				"default", defaultValue,
			)
		}
		return defaultValue
	}
	return value
}

// getBoolEnvOrDefault returns the environment variable as bool or the default.
func (m *Manager) getBoolEnvOrDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		m.logger.Info("Invalid boolean value for configuration, using default",
			"key", key,
			"value", value,
			"default", defaultValue,
		)
		return defaultValue
	}
	return parsed
}

// DefaultResourceQuota returns the default resource quota configuration.
func DefaultResourceQuota() ResourceQuotaConfig {
	return ResourceQuotaConfig{
		CPURequests:    resource.MustParse(constants.DefaultQuotaCPURequests),
		CPULimits:      resource.MustParse(constants.DefaultQuotaCPULimits),
		MemoryRequests: resource.MustParse(constants.DefaultQuotaMemoryRequests),
		MemoryLimits:   resource.MustParse(constants.DefaultQuotaMemoryLimits),
		PVCCount:       5,
		PodCount:       20,
	}
}

// DefaultLimitRange returns the default limit range configuration.
func DefaultLimitRange() LimitRangeConfig {
	return LimitRangeConfig{
		DefaultCPU:    resource.MustParse(constants.DefaultLimitCPU),
		DefaultMemory: resource.MustParse(constants.DefaultLimitMemory),
		RequestCPU:    resource.MustParse(constants.DefaultRequestCPU),
		RequestMemory: resource.MustParse(constants.DefaultRequestMemory),
		MaxCPU:        resource.MustParse(constants.DefaultMaxCPU),
		MaxMemory:     resource.MustParse(constants.DefaultMaxMemory),
		MinCPU:        resource.MustParse(constants.DefaultMinCPU),
		MinMemory:     resource.MustParse(constants.DefaultMinMemory),
	}
}

// IsInitialized returns true if the configuration has been loaded.
func (m *Manager) IsInitialized() bool {
	return m.config != nil
}
