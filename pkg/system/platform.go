package system

import "sambo/pkg/platform"

// Platform defines the interface for OS-specific operations
type Platform interface {
	IsLinux() bool
	IsMacOS() bool
	SambaConfigPath() string
	SambaConfigDir() string
	ServiceManager() string
}

// RealPlatform implements Platform using the platform package
type RealPlatform struct{}

func (p *RealPlatform) IsLinux() bool {
	return platform.IsLinux()
}

func (p *RealPlatform) IsMacOS() bool {
	return platform.IsMacOS()
}

func (p *RealPlatform) SambaConfigPath() string {
	return platform.SambaConfigPath()
}

func (p *RealPlatform) SambaConfigDir() string {
	return platform.SambaConfigDir()
}

func (p *RealPlatform) ServiceManager() string {
	return platform.ServiceManager()
}
