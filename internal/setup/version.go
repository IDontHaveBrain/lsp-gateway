package setup

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Original   string
}

type CompatibilityResult struct {
	IsCompatible       bool
	Score              int // 0-100 compatibility score
	MinimumVersion     string
	RecommendedVersion string
	UpgradePath        string
	Notes              []string
}

type VersionChecker struct {
	MinVersions         map[string]string
	RecommendedVersions map[string]string
}

func NewVersionChecker() *VersionChecker {
	return &VersionChecker{
		MinVersions: map[string]string{
			"go":     "1.19.0",
			"python": "3.9.0",
			"nodejs": "22.0.0",
			"java":   "17.0.0",
		},
		RecommendedVersions: map[string]string{
			"go":     "1.21.0",
			"python": "3.10.0",
			"nodejs": "22.0.0",
			"java":   "21.0.0",
		},
	}
}

func (v *VersionChecker) SetMinVersion(runtime, version string) {
	v.MinVersions[runtime] = version
}

func (v *VersionChecker) SetRecommendedVersion(runtime, version string) {
	v.RecommendedVersions[runtime] = version
}

func (v *VersionChecker) GetMinVersion(runtime string) (string, bool) {
	version, exists := v.MinVersions[runtime]
	return version, exists
}

func (v *VersionChecker) GetRecommendedVersion(runtime string) (string, bool) {
	version, exists := v.RecommendedVersions[runtime]
	return version, exists
}

func (v *VersionChecker) IsCompatible(runtime, version string) bool {
	result := v.CheckCompatibility(runtime, version)
	return result.IsCompatible
}

func (v *VersionChecker) CheckCompatibility(runtime, version string) *CompatibilityResult {
	result := &CompatibilityResult{
		IsCompatible: false,
		Score:        0,
		Notes:        []string{},
	}

	minVersion, minExists := v.MinVersions[runtime]
	recVersion, recExists := v.RecommendedVersions[runtime]

	if !minExists {
		result.IsCompatible = true
		result.Score = 100
		result.Notes = append(result.Notes, "No minimum version requirement specified")
		return result
	}

	result.MinimumVersion = minVersion
	if recExists {
		result.RecommendedVersion = recVersion
	}

	currentVersion, err := ParseVersion(version)
	if err != nil {
		result.Notes = append(result.Notes, fmt.Sprintf("Failed to parse version: %v", err))
		result.UpgradePath = fmt.Sprintf("Please provide a valid version format. Expected format: major.minor.patch (e.g., %s)", minVersion)
		return result
	}

	requiredVersion, err := ParseVersion(minVersion)
	if err != nil {
		result.Notes = append(result.Notes, fmt.Sprintf("Failed to parse minimum version: %v", err))
		return result
	}

	comparison := CompareVersions(currentVersion, requiredVersion)

	if comparison < 0 {
		result.IsCompatible = false
		result.Score = 0
		result.UpgradePath = fmt.Sprintf("Upgrade from %s to at least %s", version, minVersion)
		result.Notes = append(result.Notes, "Version below minimum requirement")
		return result
	}

	result.IsCompatible = true

	if recExists {
		recommendedVersion, err := ParseVersion(recVersion)
		if err == nil {
			recComparison := CompareVersions(currentVersion, recommendedVersion)
			if recComparison >= 0 {
				result.Score = 100
				result.Notes = append(result.Notes, "Meets or exceeds recommended version")
			} else {
				result.Score = 75
				result.Notes = append(result.Notes, fmt.Sprintf("Consider upgrading to recommended version %s", recVersion))
				result.UpgradePath = fmt.Sprintf("Upgrade from %s to %s for optimal performance", version, recVersion)
			}
		}
	} else {
		result.Score = 85 // Good but no recommendation available
		result.Notes = append(result.Notes, "Meets minimum requirements")
	}

	return result
}

func ParseVersion(versionString string) (*Version, error) {
	if err := validateVersionInput(versionString); err != nil {
		return nil, err
	}

	original := versionString
	cleaned := cleanVersionString(versionString)

	matches, err := matchVersionPattern(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s (cleaned: %s)", versionString, cleaned)
	}

	components, err := extractVersionComponents(matches)
	if err != nil {
		return nil, err
	}

	return buildVersionObject(components, original), nil
}

func validateVersionInput(versionString string) error {
	if versionString == "" {
		return fmt.Errorf("empty version string")
	}

	if strings.Contains(strings.ToLower(versionString), "devel") {
		return fmt.Errorf("development version not supported: %s", versionString)
	}

	return nil
}

func cleanVersionString(versionString string) string {
	cleaned := versionString

	cleaned = removePrefixes(cleaned)
	cleaned = handleUnderscoreSeparator(cleaned)
	cleaned = normalizeShortVersions(cleaned, versionString)

	return cleaned
}

func removePrefixes(cleaned string) string {
	prefixes := []string{"go", "v", "version", "release", "python", "node", "nodejs", "java", "openjdk"}

	for _, prefix := range prefixes {
		cleaned = removeMatchingPrefix(cleaned, prefix)
	}

	return cleaned
}

func removeMatchingPrefix(cleaned, prefix string) string {
	lowerCleaned := strings.ToLower(cleaned)
	if !strings.HasPrefix(lowerCleaned, prefix) {
		return cleaned
	}

	remaining := cleaned[len(prefix):]
	if shouldRemovePrefix(remaining) {
		remaining = strings.TrimLeft(remaining, " \t-")
		return remaining
	}

	return cleaned
}

func shouldRemovePrefix(remaining string) bool {
	if len(remaining) == 0 {
		return true
	}

	prefixChars := []string{"-", " ", "\t"}
	for _, char := range prefixChars {
		if strings.HasPrefix(remaining, char) {
			return true
		}
	}

	return regexp.MustCompile(`^\d`).MatchString(remaining)
}

func handleUnderscoreSeparator(cleaned string) string {
	if strings.Contains(cleaned, "_") {
		parts := strings.Split(cleaned, "_")
		return parts[0]
	}
	return cleaned
}

func normalizeShortVersions(cleaned, original string) string {
	if regexp.MustCompile(`^\d+$`).MatchString(cleaned) && original != cleaned {
		return cleaned + ".0.0"
	}
	return cleaned
}

func matchVersionPattern(cleaned string) ([]string, error) {
	re := regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?(?:-([a-zA-Z0-9\.\-]+))?(?:\+([a-zA-Z0-9\.\-]+))?$|^(\d+)\.(\d+)\.(\d+)([a-zA-Z]+\d*)(?:\+([a-zA-Z0-9\.\-]+))?$`)
	matches := re.FindStringSubmatch(cleaned)

	if matches == nil {
		return nil, fmt.Errorf("no match found")
	}

	return matches, nil
}

type versionComponents struct {
	major, minor, patch int
	prerelease, build   string
}

func extractVersionComponents(matches []string) (*versionComponents, error) {
	if matches[1] != "" {
		return extractStandardVersionComponents(matches)
	} else if matches[6] != "" {
		return extractAlternativeVersionComponents(matches)
	}

	return nil, fmt.Errorf("no valid version pattern found")
}

func extractStandardVersionComponents(matches []string) (*versionComponents, error) {
	major, err := parseVersionNumber(matches[1], "major")
	if err != nil {
		return nil, err
	}

	minor, err := parseVersionNumber(matches[2], "minor")
	if err != nil {
		return nil, err
	}

	patch := 0
	if matches[3] != "" {
		patch, err = parseVersionNumber(matches[3], "patch")
		if err != nil {
			return nil, err
		}
	}

	return &versionComponents{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: matches[4],
		build:      matches[5],
	}, nil
}

func extractAlternativeVersionComponents(matches []string) (*versionComponents, error) {
	major, err := parseVersionNumber(matches[6], "major")
	if err != nil {
		return nil, err
	}

	minor, err := parseVersionNumber(matches[7], "minor")
	if err != nil {
		return nil, err
	}

	patch, err := parseVersionNumber(matches[8], "patch")
	if err != nil {
		return nil, err
	}

	return &versionComponents{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: matches[9],
		build:      matches[10],
	}, nil
}

func parseVersionNumber(numberStr, component string) (int, error) {
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s version '%s': %v", component, numberStr, err)
	}
	return number, nil
}

func buildVersionObject(components *versionComponents, original string) *Version {
	return &Version{
		Major:      components.major,
		Minor:      components.minor,
		Patch:      components.patch,
		Prerelease: components.prerelease,
		Build:      components.build,
		Original:   original,
	}
}

func CompareVersions(v1, v2 *Version) int {
	if v1.Major < v2.Major {
		return -1
	}
	if v1.Major > v2.Major {
		return 1
	}

	if v1.Minor < v2.Minor {
		return -1
	}
	if v1.Minor > v2.Minor {
		return 1
	}

	if v1.Patch < v2.Patch {
		return -1
	}
	if v1.Patch > v2.Patch {
		return 1
	}

	if v1.Prerelease == "" && v2.Prerelease != "" {
		return 1
	}
	if v1.Prerelease != "" && v2.Prerelease == "" {
		return -1
	}

	if v1.Prerelease != "" && v2.Prerelease != "" {
		if v1.Prerelease < v2.Prerelease {
			return -1
		}
		if v1.Prerelease > v2.Prerelease {
			return 1
		}
	}

	return 0
}

func (v *Version) String() string {
	result := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		result += "-" + v.Prerelease
	}
	if v.Build != "" {
		result += "+" + v.Build
	}
	return result
}

func (v *Version) ShortString() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v *Version) IsGreaterOrEqualTo(other *Version) bool {
	return CompareVersions(v, other) >= 0
}

func (v *Version) IsGreaterThan(other *Version) bool {
	return CompareVersions(v, other) > 0
}

func (v *Version) IsEqualTo(other *Version) bool {
	return CompareVersions(v, other) == 0
}

func (v *Version) IsLessThan(other *Version) bool {
	return CompareVersions(v, other) < 0
}

func (v *Version) IsLessOrEqualTo(other *Version) bool {
	return CompareVersions(v, other) <= 0
}

func (v *Version) IsPrerelease() bool {
	return v.Prerelease != ""
}

func (v *Version) HasBuildMetadata() bool {
	return v.Build != ""
}

func (v *Version) IsStable() bool {
	return v.Prerelease == ""
}

func GetUpgradePath(from, to string) (string, error) {
	fromVersion, err := ParseVersion(from)
	if err != nil {
		return "", fmt.Errorf("invalid source version: %v", err)
	}

	toVersion, err := ParseVersion(to)
	if err != nil {
		return "", fmt.Errorf("invalid target version: %v", err)
	}

	comparison := CompareVersions(fromVersion, toVersion)
	if comparison == 0 {
		return "Already at target version", nil
	} else if comparison > 0 {
		return fmt.Sprintf("Downgrade from %s to %s", fromVersion.ShortString(), toVersion.ShortString()), nil
	} else {
		majorDiff := toVersion.Major - fromVersion.Major
		minorDiff := toVersion.Minor - fromVersion.Minor
		patchDiff := toVersion.Patch - fromVersion.Patch

		if majorDiff > 0 {
			return fmt.Sprintf("Major upgrade from %s to %s (may contain breaking changes)", fromVersion.ShortString(), toVersion.ShortString()), nil
		} else if minorDiff > 0 {
			return fmt.Sprintf("Minor upgrade from %s to %s (new features, backward compatible)", fromVersion.ShortString(), toVersion.ShortString()), nil
		} else if patchDiff > 0 {
			return fmt.Sprintf("Patch upgrade from %s to %s (bug fixes)", fromVersion.ShortString(), toVersion.ShortString()), nil
		}

		return fmt.Sprintf("Upgrade from %s to %s", fromVersion.ShortString(), toVersion.ShortString()), nil
	}
}
