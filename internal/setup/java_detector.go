package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
)

type JavaRuntimeInfo struct {
	*RuntimeInfo
	JavaHome        string   // JAVA_HOME environment variable value
	JavaHomeValid   bool     // Whether JAVA_HOME is valid and points to correct installation
	Distribution    string   // Java distribution (OpenJDK, Oracle JDK, etc.)
	IsJDK           bool     // Whether this is a JDK (vs JRE only)
	DevToolsFound   []string // Available development tools (javac, jar, etc.)
	DevToolsMissing []string // Missing development tools
	ClassPath       string   // Default classpath
	JVMFlags        []string // Detected JVM flags and capabilities
}

type JavaDetector struct {
	executor       platform.CommandExecutor
	versionChecker *VersionChecker
}

func NewJavaDetector(versionChecker *VersionChecker) *JavaDetector {
	return &JavaDetector{
		executor:       platform.NewCommandExecutor(),
		versionChecker: versionChecker,
	}
}

func (d *JavaDetector) DetectJava() (*JavaRuntimeInfo, error) {
	javaInfo := &JavaRuntimeInfo{
		RuntimeInfo: &RuntimeInfo{
			Name:       "java",
			Installed:  false,
			Version:    "",
			Compatible: false,
			Path:       "",
			Issues:     []string{},
		},
		DevToolsFound:   []string{},
		DevToolsMissing: []string{},
		JVMFlags:        []string{},
	}

	if !d.executor.IsCommandAvailable("java") {
		javaInfo.Issues = append(javaInfo.Issues, "Java runtime not found in PATH")
		return javaInfo, nil
	}

	javaPath, err := d.getJavaExecutablePath()
	if err != nil {
		javaInfo.Issues = append(javaInfo.Issues, fmt.Sprintf("Failed to locate java executable: %v", err))
		return javaInfo, nil
	}
	javaInfo.Path = javaPath
	javaInfo.Installed = true

	version, distribution, err := d.detectJavaVersion()
	if err != nil {
		javaInfo.Issues = append(javaInfo.Issues, fmt.Sprintf("Failed to detect Java version: %v", err))
		return javaInfo, nil
	}
	javaInfo.Version = version
	javaInfo.Distribution = distribution

	javaInfo.Compatible = d.versionChecker.IsCompatible("java", version)
	if !javaInfo.Compatible {
		minVersion, _ := d.versionChecker.GetMinVersion("java")
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("Java version %s does not meet minimum requirement %s", version, minVersion))
	}

	d.validateJavaHome(javaInfo)

	d.detectJDKCapabilities(javaInfo)

	d.performPlatformSpecificValidation(javaInfo)

	d.validateClasspathFunctionality(javaInfo)

	return javaInfo, nil
}

func (d *JavaDetector) getJavaExecutablePath() (string, error) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"where", "java"}
	} else {
		cmd = []string{"which", "java"}
	}

	result, err := d.executor.Execute(cmd[0], cmd[1:], 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to locate java executable: %w", err)
	}

	path := strings.TrimSpace(result.Stdout)
	if path == "" {
		return "", fmt.Errorf("java executable not found in PATH")
	}

	if runtime.GOOS != "windows" {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
	}

	return path, nil
}

func (d *JavaDetector) detectJavaVersion() (version, distribution string, err error) {
	result, err := d.executor.Execute("java", []string{"-version"}, 10*time.Second)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute java -version: %w", err)
	}

	output := result.Stderr
	if output == "" {
		output = result.Stdout
	}

	version, distribution = d.parseJavaVersionOutput(output)
	if version == "" {
		return "", "", fmt.Errorf("could not parse Java version from output: %s", output)
	}

	return version, distribution, nil
}

func (d *JavaDetector) parseJavaVersionOutput(output string) (version, distribution string) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return "", ""
	}

	firstLine := strings.TrimSpace(lines[0])

	distribution = "Unknown"
	lowerLine := strings.ToLower(firstLine)
	allLines := strings.ToLower(output)

	switch {
	case strings.Contains(allLines, "temurin"):
		distribution = "Eclipse Temurin"
	case strings.Contains(allLines, "corretto"):
		distribution = "Amazon Corretto"
	case strings.Contains(allLines, "zulu"):
		distribution = "Azul Zulu"
	case strings.Contains(allLines, "graalvm"):
		distribution = "GraalVM"
	case strings.Contains(lowerLine, "ibm"):
		distribution = "IBM JDK"
	case strings.Contains(lowerLine, "oracle"):
		distribution = "Oracle JDK"
	case strings.Contains(lowerLine, "java version"):
		distribution = "Oracle JDK"
	case strings.Contains(lowerLine, "openjdk"):
		distribution = "OpenJDK"
	}

	versionRegex := regexp.MustCompile(`"(\d+(?:\.\d+)*(?:_\d+)?)"`)
	matches := versionRegex.FindStringSubmatch(firstLine)
	if len(matches) > 1 {
		rawVersion := matches[1]

		if strings.HasPrefix(rawVersion, "1.") {
			parts := strings.SplitN(rawVersion, ".", 3)
			if len(parts) >= 2 {
				majorStr := parts[1]
				remaining := ""
				if len(parts) > 2 {
					remaining = "." + parts[2]
				}
				remaining = strings.ReplaceAll(remaining, "_", ".")
				version = majorStr + remaining
			}
		} else {
			version = strings.ReplaceAll(rawVersion, "_", ".")
		}
	}

	return version, distribution
}

func (d *JavaDetector) validateJavaHome(javaInfo *JavaRuntimeInfo) {
	javaHome := os.Getenv("JAVA_HOME")
	javaInfo.JavaHome = javaHome

	if javaHome == "" {
		javaInfo.Issues = append(javaInfo.Issues, "JAVA_HOME environment variable is not set")
		return
	}

	if _, err := os.Stat(javaHome); os.IsNotExist(err) {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("JAVA_HOME points to non-existent directory: %s", javaHome))
		return
	}

	javaExecutable := filepath.Join(javaHome, "bin", "java")
	if runtime.GOOS == "windows" {
		javaExecutable += ".exe"
	}

	if _, err := os.Stat(javaExecutable); os.IsNotExist(err) {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("JAVA_HOME does not contain valid Java installation: %s", javaHome))
		return
	}

	result, err := d.executor.Execute(javaExecutable, []string{"-version"}, 5*time.Second)
	if err != nil {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("JAVA_HOME java executable is not functional: %v", err))
		return
	}

	javaHomeVersion, _ := d.parseJavaVersionOutput(result.Stderr)
	if javaHomeVersion == "" {
		javaHomeVersion, _ = d.parseJavaVersionOutput(result.Stdout)
	}

	if javaHomeVersion != javaInfo.Version {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("JAVA_HOME java version (%s) differs from PATH java version (%s)",
				javaHomeVersion, javaInfo.Version))
		return
	}

	javaInfo.JavaHomeValid = true
}

func (d *JavaDetector) detectJDKCapabilities(javaInfo *JavaRuntimeInfo) {
	jdkTools := []string{"javac", "jar", "javadoc", "jdb", "jconsole", "jps"}

	foundTools := []string{}
	missingTools := []string{}

	for _, tool := range jdkTools {
		if d.executor.IsCommandAvailable(tool) {
			foundTools = append(foundTools, tool)
		} else {
			missingTools = append(missingTools, tool)
		}
	}

	javaInfo.DevToolsFound = foundTools
	javaInfo.DevToolsMissing = missingTools

	javaInfo.IsJDK = d.executor.IsCommandAvailable("javac")

	if !javaInfo.IsJDK {
		javaInfo.Issues = append(javaInfo.Issues,
			"JRE detected but JDK required for Java Language Server (javac not found)")

		if javaInfo.JavaHome != "" {
			javacPath := filepath.Join(javaInfo.JavaHome, "bin", "javac")
			if runtime.GOOS == "windows" {
				javacPath += ".exe"
			}

			if _, err := os.Stat(javacPath); err == nil {
				javaInfo.Issues = append(javaInfo.Issues,
					"JDK tools found in JAVA_HOME but not in PATH - check PATH configuration")
			}
		}
	}

	if javaInfo.IsJDK {
		result, err := d.executor.Execute("javac", []string{"-version"}, 5*time.Second)
		if err == nil {
			javacOutput := result.Stderr
			if javacOutput == "" {
				javacOutput = result.Stdout
			}

			javacVersion := d.extractJavacVersion(javacOutput)
			if javacVersion != "" && javacVersion != javaInfo.Version {
				javaInfo.Issues = append(javaInfo.Issues,
					fmt.Sprintf("javac version (%s) differs from java version (%s)",
						javacVersion, javaInfo.Version))
			}
		}
	}
}

func (d *JavaDetector) extractJavacVersion(output string) string {
	versionRegex := regexp.MustCompile(`javac\s+(\d+(?:\.\d+)*(?:_\d+)?)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		rawVersion := matches[1]
		if strings.HasPrefix(rawVersion, "1.") {
			parts := strings.SplitN(rawVersion, ".", 3)
			if len(parts) >= 2 {
				majorStr := parts[1]
				remaining := ""
				if len(parts) > 2 {
					remaining = "." + parts[2]
				}
				remaining = strings.ReplaceAll(remaining, "_", ".")
				return majorStr + remaining
			}
		}
		return strings.ReplaceAll(rawVersion, "_", ".")
	}
	return ""
}

func (d *JavaDetector) performPlatformSpecificValidation(javaInfo *JavaRuntimeInfo) {
	switch runtime.GOOS {
	case "windows":
		d.validateWindowsJava(javaInfo)
	case "darwin":
		d.validateMacOSJava(javaInfo)
	case "linux":
		d.validateLinuxJava(javaInfo)
	}
}

func (d *JavaDetector) validateWindowsJava(javaInfo *JavaRuntimeInfo) {

	systemJava := filepath.Join(os.Getenv("WINDIR"), "System32", "java.exe")
	if _, err := os.Stat(systemJava); err == nil {
		if strings.Contains(javaInfo.Path, "System32") {
			javaInfo.Issues = append(javaInfo.Issues,
				"Java detected in System32 - this may be a redirect to Oracle installer")
		}
	}

	commonPaths := []string{
		"C:\\Program Files\\Java",
		"C:\\Program Files (x86)\\Java",
		"C:\\Program Files\\Eclipse Adoptium",
		"C:\\Program Files\\Eclipse Foundation",
		"C:\\Program Files\\Amazon Corretto",
	}

	installedVersions := []string{}
	for _, basePath := range commonPaths {
		if entries, err := os.ReadDir(basePath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					installedVersions = append(installedVersions, filepath.Join(basePath, entry.Name()))
				}
			}
		}
	}

	if len(installedVersions) > 1 {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("Multiple Java installations detected: %s", strings.Join(installedVersions, ", ")))
	}
}

func (d *JavaDetector) validateMacOSJava(javaInfo *JavaRuntimeInfo) {
	result, err := d.executor.Execute("/usr/libexec/java_home", []string{}, 5*time.Second)
	if err == nil {
		systemJavaHome := strings.TrimSpace(result.Stdout)
		if systemJavaHome != "" && systemJavaHome != javaInfo.JavaHome {
			javaInfo.Issues = append(javaInfo.Issues,
				fmt.Sprintf("System java_home (%s) differs from JAVA_HOME (%s)",
					systemJavaHome, javaInfo.JavaHome))
		}
	}

	brewJavaPath := "/opt/homebrew/opt/openjdk"
	if _, err := os.Stat(brewJavaPath); err == nil {
		javaInfo.JVMFlags = append(javaInfo.JVMFlags, "homebrew-installed")
	}

	systemJavaPath := "/System/Library/Java/JavaVirtualMachines"
	if _, err := os.Stat(systemJavaPath); err == nil {
		if strings.Contains(javaInfo.Path, systemJavaPath) {
			javaInfo.Issues = append(javaInfo.Issues,
				"Using macOS system Java - consider installing a modern Java distribution")
		}
	}
}

func (d *JavaDetector) validateLinuxJava(javaInfo *JavaRuntimeInfo) {
	packageManagers := map[string][]string{
		"dpkg":   {"-l", "openjdk*"},
		"rpm":    {"-qa", "java*"},
		"pacman": {"-Q", "jdk*"},
	}

	for pm, args := range packageManagers {
		if d.executor.IsCommandAvailable(pm) {
			result, err := d.executor.Execute(pm, args, 5*time.Second)
			if err == nil && result.Stdout != "" {
				javaInfo.JVMFlags = append(javaInfo.JVMFlags, fmt.Sprintf("package-manager-%s", pm))
			}
		}
	}

	sdkmanPath := filepath.Join(os.Getenv("HOME"), ".sdkman", "candidates", "java")
	if _, err := os.Stat(sdkmanPath); err == nil {
		javaInfo.JVMFlags = append(javaInfo.JVMFlags, "sdkman-managed")
	}

	if d.executor.IsCommandAvailable("update-alternatives") {
		result, err := d.executor.Execute("update-alternatives", []string{"--display", "java"}, 5*time.Second)
		if err == nil && strings.Contains(result.Stdout, "link currently points to") {
			javaInfo.JVMFlags = append(javaInfo.JVMFlags, "alternatives-managed")
		}
	}
}

func (d *JavaDetector) validateClasspathFunctionality(javaInfo *JavaRuntimeInfo) {
	_, err := d.executor.Execute("java", []string{"-cp", ".", "-version"}, 5*time.Second)
	if err != nil {
		javaInfo.Issues = append(javaInfo.Issues,
			fmt.Sprintf("Java classpath functionality test failed: %v", err))
		return
	}

	classpathResult, err := d.executor.Execute("java", []string{"-XshowSettings:properties", "-version"}, 5*time.Second)
	if err == nil {
		lines := strings.Split(classpathResult.Stderr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "java.class.path") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					javaInfo.ClassPath = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}

	if d.executor.IsCommandAvailable("jar") {
		jarResult, err := d.executor.Execute("jar", []string{"--version"}, 3*time.Second)
		if err == nil && jarResult.ExitCode == 0 {
			javaInfo.JVMFlags = append(javaInfo.JVMFlags, "jar-functional")
		} else {
			javaInfo.Issues = append(javaInfo.Issues, "jar command found but not functional")
		}
	}
}
