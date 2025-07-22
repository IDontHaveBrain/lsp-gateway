package platform

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformLinux   Platform = "linux"
	PlatformMacOS   Platform = "darwin"
	PlatformUnknown Platform = "unknown"
)

type Architecture string

const (
	ArchAMD64   Architecture = "amd64"
	ArchARM64   Architecture = "arm64"
	Arch386     Architecture = "386"
	ArchARM     Architecture = "arm"
	ArchUnknown Architecture = "unknown"
)

// GetCurrentPlatform returns the current operating system platform
// detected at runtime (Windows, Linux, Darwin/macOS, or Unknown).
func GetCurrentPlatform() Platform {
	switch runtime.GOOS {
	case string(PlatformWindows):
		return PlatformWindows
	case string(PlatformLinux):
		return PlatformLinux
	case PLATFORM_DARWIN:
		return PlatformMacOS
	default:
		return PlatformUnknown
	}
}

// GetCurrentArchitecture returns the current system architecture
// detected at runtime (AMD64, ARM64, 386, ARM, or Unknown).
func GetCurrentArchitecture() Architecture {
	switch runtime.GOARCH {
	case string(ArchAMD64):
		return ArchAMD64
	case string(ArchARM64):
		return ArchARM64
	case string(Arch386):
		return Arch386
	case string(ArchARM):
		return ArchARM
	default:
		return ArchUnknown
	}
}

func IsWindows() bool {
	return GetCurrentPlatform() == PlatformWindows
}

func IsLinux() bool {
	return GetCurrentPlatform() == PlatformLinux
}

func IsMacOS() bool {
	return GetCurrentPlatform() == PlatformMacOS
}

func IsUnix() bool {
	platform := GetCurrentPlatform()
	return platform == PlatformLinux || platform == PlatformMacOS
}

func GetHomeDirectory() (string, error) {
	if IsWindows() {
		if home := os.Getenv("USERPROFILE"); home != "" {
			return home, nil
		}

		drive := os.Getenv("HOMEDRIVE")
		path := os.Getenv("HOMEPATH")
		if drive != "" && path != "" {
			return drive + path, nil
		}

		return "", fmt.Errorf("could not determine home directory on Windows")
	}

	if home := os.Getenv(ENV_VAR_HOME); home != "" {
		return home, nil
	}

	return "", fmt.Errorf("could not determine home directory")
}

func GetTempDirectory() string {
	if dir := os.Getenv("TMPDIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("TMP"); dir != "" {
		return dir
	}
	if dir := os.Getenv("TEMP"); dir != "" {
		return dir
	}

	if IsWindows() {
		return "C:\\Windows\\Temp"
	}

	return "/tmp"
}

func GetExecutableExtension() string {
	if IsWindows() {
		return EXECUTABLE_EXTENSION
	}
	return ""
}

func GetPathSeparator() string {
	if IsWindows() {
		return "\\"
	}
	return "/"
}

func GetPathListSeparator() string {
	if IsWindows() {
		return ";"
	}
	return ":"
}

func (p Platform) String() string {
	return string(p)
}

func (a Architecture) String() string {
	return string(a)
}

func GetPlatformString() string {
	return string(GetCurrentPlatform()) + "-" + string(GetCurrentArchitecture())
}

func SupportsShell(shell string) bool {
	shellLower := strings.ToLower(shell)

	if IsWindows() {
		windowsShells := []string{"cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe"}
		for _, supported := range windowsShells {
			if shellLower == supported || strings.HasSuffix(shellLower, "/"+supported) || strings.HasSuffix(shellLower, "\\"+supported) {
				return true
			}
		}
		return false
	}

	unixShells := []string{"sh", "bash", "zsh", "dash", "ksh", "fish"}
	for _, supported := range unixShells {
		if shellLower == supported || strings.HasSuffix(shellLower, "/"+supported) {
			return true
		}
	}

	return false
}

type LinuxDistribution string

const (
	DistributionUbuntu   LinuxDistribution = "ubuntu"
	DistributionDebian   LinuxDistribution = "debian"
	DistributionFedora   LinuxDistribution = "fedora"
	DistributionCentOS   LinuxDistribution = "centos"
	DistributionRHEL     LinuxDistribution = "rhel"
	DistributionArch     LinuxDistribution = "arch"
	DistributionOpenSUSE LinuxDistribution = "opensuse"
	DistributionAlpine   LinuxDistribution = "alpine"
	DistributionUnknown  LinuxDistribution = "unknown"
)

type LinuxInfo struct {
	Distribution LinuxDistribution
	Version      string
	Name         string
	ID           string
	IDLike       []string
}

func DetectLinuxDistribution() (*LinuxInfo, error) {
	if !IsLinux() {
		return nil, fmt.Errorf("not running on Linux")
	}

	info := &LinuxInfo{
		Distribution: DistributionUnknown,
	}

	if osRelease, err := readOSRelease(); err == nil {
		parseOSRelease(osRelease, info)
		return info, nil
	}

	if lsbRelease, err := readLSBRelease(); err == nil {
		parseLSBRelease(lsbRelease, info)
		return info, nil
	}

	if err := detectFromDistributionFiles(info); err == nil {
		return info, nil
	}

	if err := detectFromUname(info); err == nil {
		return info, nil
	}

	return info, fmt.Errorf("could not detect Linux distribution")
}

func readOSRelease() (map[string]string, error) {
	return readReleaseFile(OS_RELEASE_FILE)
}

func readLSBRelease() (map[string]string, error) {
	return readReleaseFile("/etc/lsb-release")
}

func readReleaseFile(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}

		result[key] = value
	}

	return result, nil
}

func parseOSRelease(data map[string]string, info *LinuxInfo) {
	if id, exists := data["ID"]; exists {
		info.ID = id
		info.Distribution = mapIDToDistribution(id)
	}

	if name, exists := data["NAME"]; exists {
		info.Name = name
	}

	if version, exists := data["VERSION_ID"]; exists {
		info.Version = version
	}

	if idLike, exists := data["ID_LIKE"]; exists {
		info.IDLike = strings.Fields(idLike)
	}
}

func parseLSBRelease(data map[string]string, info *LinuxInfo) {
	if id, exists := data["DISTRIB_ID"]; exists {
		info.ID = strings.ToLower(id)
		info.Distribution = mapIDToDistribution(info.ID)
	}

	if release, exists := data["DISTRIB_RELEASE"]; exists {
		info.Version = release
	}

	if description, exists := data["DISTRIB_DESCRIPTION"]; exists {
		info.Name = description
	}
}

func mapIDToDistribution(id string) LinuxDistribution {
	id = strings.ToLower(id)

	switch id {
	case string(DistributionUbuntu):
		return DistributionUbuntu
	case "debian":
		return DistributionDebian
	case string(DistributionFedora):
		return DistributionFedora
	case "centos":
		return DistributionCentOS
	case "rhel", "red", "redhat":
		return DistributionRHEL
	case "arch":
		return DistributionArch
	case "opensuse", "suse":
		return DistributionOpenSUSE
	case "alpine":
		return DistributionAlpine
	default:
		return DistributionUnknown
	}
}

func detectFromDistributionFiles(info *LinuxInfo) error {
	files := map[string]LinuxDistribution{
		"/etc/debian_version": DistributionDebian,
		"/etc/redhat-release": DistributionRHEL,
		"/etc/centos-release": DistributionCentOS,
		"/etc/fedora-release": DistributionFedora,
		"/etc/arch-release":   DistributionArch,
		"/etc/SuSE-release":   DistributionOpenSUSE,
		"/etc/alpine-release": DistributionAlpine,
	}

	for file, dist := range files {
		if _, err := os.Stat(file); err == nil {
			info.Distribution = dist

			if content, err := os.ReadFile(file); err == nil {
				info.Version = strings.TrimSpace(string(content))
			}

			return nil
		}
	}

	return fmt.Errorf("no distribution files found")
}

func detectFromUname(info *LinuxInfo) error {
	executor := NewCommandExecutor()
	result, err := executor.Execute("uname", []string{"-a"}, 10*time.Second)
	if err != nil {
		return err
	}

	output := strings.ToLower(result.Stdout)

	if strings.Contains(output, string(DistributionUbuntu)) {
		info.Distribution = DistributionUbuntu
	} else if strings.Contains(output, "debian") {
		info.Distribution = DistributionDebian
	} else if strings.Contains(output, string(DistributionFedora)) {
		info.Distribution = DistributionFedora
	} else if strings.Contains(output, "centos") {
		info.Distribution = DistributionCentOS
	} else if strings.Contains(output, "red hat") {
		info.Distribution = DistributionRHEL
	} else {
		return fmt.Errorf("could not determine distribution from uname")
	}

	return nil
}

func GetPreferredPackageManagers(distribution LinuxDistribution) []string {
	switch distribution {
	case DistributionUbuntu, DistributionDebian:
		return []string{"apt"}
	case DistributionFedora:
		return []string{"dnf"}
	case DistributionCentOS, DistributionRHEL:
		return []string{"dnf", "yum"}
	case DistributionArch:
		return []string{"pacman"}
	case DistributionOpenSUSE:
		return []string{"zypper"}
	case DistributionAlpine:
		return []string{"apk"}
	default:
		return []string{"apt", "dnf", "yum"}
	}
}

func (d LinuxDistribution) String() string {
	return string(d)
}
