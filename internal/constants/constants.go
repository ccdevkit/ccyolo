package constants

import "path/filepath"

// Container paths
const (
	ContainerHome      = "/home/claude"
	ContainerBridgeDir = "/home/claude/.ccyolo-bridge"
	ContainerTmpDir    = "/tmp"
	HijackerDir        = "/opt/ccyolo/bin"
)

// Container config file names
const (
	ProxyConfigFile  = "ccyolo-proxy.json"
	SettingsFile     = "ccyolo-settings.json"
	SystemPromptFile = "ccyolo-system-prompt.md"
)

// ContainerProxyConfigPath returns the full path to the proxy config in the container
func ContainerProxyConfigPath() string {
	return filepath.Join(ContainerTmpDir, ProxyConfigFile)
}

// ContainerSettingsPath returns the full path to settings in the container
func ContainerSettingsPath() string {
	return filepath.Join(ContainerTmpDir, SettingsFile)
}

// ContainerSystemPromptPath returns the full path to system prompt in the container
func ContainerSystemPromptPath() string {
	return filepath.Join(ContainerBridgeDir, SystemPromptFile)
}

// Host directory names (relative to home)
const (
	CckitDirName   = ".cckit/ccyolo"
	ClaudeDirName  = ".claude"
	BridgeDirName  = ".ccyolo-bridge"
	ClaudeJsonFile = ".claude.json"
	SettingsJson   = "settings.json"
)

// Docker configuration
const (
	DockerBaseImageRegistry = "ghcr.io/ccdevkit/ccyolo-base"
	DockerLocalImageName    = "ccyolo-local"
	DockerHostDNS           = "host.docker.internal"
	DefaultXDisplay         = ":99"
)

// Environment variable names
const (
	EnvClipboardPort = "CCYOLO_CLIP_PORT"
	EnvDisplay       = "DISPLAY"
	EnvOAuthToken    = "CLAUDE_CODE_OAUTH_TOKEN"
	EnvTerm          = "TERM"
	EnvColorTerm     = "COLORTERM"
)

// Default values
const (
	DefaultClipboardPort = "9999"
	DefaultVersion       = "dev"
)
