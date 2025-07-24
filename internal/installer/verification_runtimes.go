package installer

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (r *DefaultRuntimeInstaller) verifyPythonEnvironment(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	pythonCommands := []string{"python3", "python", "py"}
	workingCommand := ""

	for _, cmd := range pythonCommands {
		if platform.IsCommandAvailable(cmd) {
			versionResult, err := executor.Execute(cmd, []string{"--version"}, 5*time.Second)
			if err == nil && versionResult.ExitCode == 0 {
				workingCommand = cmd
				result.Metadata["python_command"] = cmd
				break
			}
		}
	}

	if workingCommand == "" {
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryInstallation,
			"No Working Python Command",
			"None of the standard Python commands (python3, python, py) are working",
			"Reinstall Python and ensure it's properly added to PATH",
			map[string]interface{}{"attempted_commands": pythonCommands})
		return
	}

	siteResult, err := executor.Execute(workingCommand, []string{"-c", "import site; print(site.getsitepackages())"}, 5*time.Second)
	if err == nil && siteResult.ExitCode == 0 {
		result.Metadata["python_site_packages"] = strings.TrimSpace(siteResult.Stdout)
	}

	versionInfoResult, err := executor.Execute(workingCommand, []string{"-c", "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}')"}, 5*time.Second)
	if err == nil && versionInfoResult.ExitCode == 0 {
		result.Metadata["python_version_detailed"] = strings.TrimSpace(versionInfoResult.Stdout)
	}

	venvResult, err := executor.Execute(workingCommand, []string{"-m", "venv", "--help"}, 5*time.Second)
	if err == nil && venvResult.ExitCode == 0 {
		result.Metadata["python_venv_supported"] = true
	} else {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
			"Virtual Environment Not Supported",
			"Python venv module is not available",
			"Install python3-venv package or upgrade Python",
			map[string]interface{}{"error": err})
	}

	if pythonPath := os.Getenv("PYTHONPATH"); pythonPath != "" {
		result.EnvironmentVars["PYTHONPATH"] = pythonPath
		result.Metadata["python_path"] = pythonPath
	}
}

func (r *DefaultRuntimeInstaller) verifyPythonPip(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	pipCommands := []string{"pip3", "pip", "python3 -m pip", "python -m pip"}
	workingPipCommand := ""

	for _, cmd := range pipCommands {
		cmdParts := strings.Fields(cmd)
		pipResult, err := executor.Execute(cmdParts[0], cmdParts[1:], 5*time.Second)
		if err == nil && pipResult.ExitCode == 0 {
			workingPipCommand = cmd
			result.Metadata["pip_command"] = cmd
			break
		}
	}

	if workingPipCommand == "" {
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			"Pip Not Available",
			"pip package manager is not installed or not working",
			"Install pip using your system package manager or python -m ensurepip",
			map[string]interface{}{"attempted_commands": pipCommands})
		return
	}

	cmdParts := strings.Fields(workingPipCommand)
	cmdParts = append(cmdParts, "--version")
	pipVersionResult, err := executor.Execute(cmdParts[0], cmdParts[1:], 5*time.Second)
	if err == nil && pipVersionResult.ExitCode == 0 {
		result.Metadata["pip_version"] = strings.TrimSpace(pipVersionResult.Stdout)
	}

	cmdParts = strings.Fields(workingPipCommand)
	cmdParts = append(cmdParts, "config", "list")
	pipConfigResult, err := executor.Execute(cmdParts[0], cmdParts[1:], 5*time.Second)
	if err == nil {
		result.Metadata["pip_config"] = strings.TrimSpace(pipConfigResult.Stdout)
	}
}

func (r *DefaultRuntimeInstaller) verifyPythonPackageInstallation(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	pipCommand, exists := result.Metadata["pip_command"].(string)
	if !exists {
		return
	}

	cmdParts := strings.Fields(pipCommand)
	cmdParts = append(cmdParts, "list", "--format=json")
	listResult, err := executor.Execute(cmdParts[0], cmdParts[1:], 10*time.Second)
	if err != nil || listResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
			"Pip List Failed",
			"Unable to list installed Python packages",
			"Check pip installation and permissions",
			map[string]interface{}{"error": err, "command": strings.Join(cmdParts, " ")})
		return
	}

	result.Metadata["pip_packages_listable"] = true
	result.Metadata["pip_packages_count"] = strings.Count(listResult.Stdout, "{")

	pythonCommand := result.Metadata["python_command"].(string)
	siteResult, err := executor.Execute(pythonCommand, []string{"-c", "import site; print(site.USER_SITE)"}, 5*time.Second)
	if err == nil && siteResult.ExitCode == 0 {
		userSite := strings.TrimSpace(siteResult.Stdout)
		result.Metadata["python_user_site"] = userSite

		if _, err := os.Stat(userSite); err == nil {
			if f, err := os.OpenFile(filepath.Join(userSite, ".test_write"), os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				if closeErr := f.Close(); closeErr != nil {
					// Ignore file close errors during site verification test
					_ = closeErr
				}
				if err := os.Remove(filepath.Join(userSite, ".test_write")); err != nil {
					// Ignore test file removal errors during site verification
					_ = err
				}
				result.Metadata["python_user_site_writable"] = true
			} else {
				r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryPermissions,
					"Python User Site Not Writable",
					fmt.Sprintf("Cannot write to Python user site directory: %s", userSite),
					"Check directory permissions or consider using virtual environments",
					map[string]interface{}{"user_site": userSite})
			}
		}
	}
}

func (r *DefaultRuntimeInstaller) verifyNodejsEnvironment(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	configResult, err := executor.Execute("node", []string{"-e", "console.log(JSON.stringify(process.config))"}, 5*time.Second)
	if err == nil && configResult.ExitCode == 0 {
		result.Metadata["nodejs_config"] = strings.TrimSpace(configResult.Stdout)
	}

	versionResult, err := executor.Execute("node", []string{"-e", "console.log(JSON.stringify(process.versions))"}, 5*time.Second)
	if err == nil && versionResult.ExitCode == 0 {
		result.Metadata["nodejs_versions"] = strings.TrimSpace(versionResult.Stdout)
	}

	if nodePath := os.Getenv("NODE_PATH"); nodePath != "" {
		result.EnvironmentVars["NODE_PATH"] = nodePath
		result.Metadata["node_path"] = nodePath
	}

	globalResult, err := executor.Execute("node", []string{"-e", "console.log(require('path').dirname(process.execPath))"}, 5*time.Second)
	if err == nil && globalResult.ExitCode == 0 {
		nodeDir := strings.TrimSpace(globalResult.Stdout)
		globalModules := filepath.Join(filepath.Dir(nodeDir), "lib", "node_modules")
		if platform.IsWindows() {
			globalModules = filepath.Join(nodeDir, "node_modules")
		}
		result.Metadata["nodejs_global_modules"] = globalModules
	}
}

func (r *DefaultRuntimeInstaller) verifyNodejsNpm(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	if !platform.IsCommandAvailable("npm") {
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			"NPM Not Available",
			"npm package manager is not installed",
			"Install npm using your system package manager or reinstall Node.js",
			map[string]interface{}{})
		return
	}

	npmVersionResult, err := executor.Execute("npm", []string{"--version"}, 5*time.Second)
	if err != nil || npmVersionResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			"NPM Version Check Failed",
			"npm is installed but version check failed",
			"Reinstall npm or check npm installation",
			map[string]interface{}{"error": err})
		return
	}

	result.Metadata["npm_version"] = strings.TrimSpace(npmVersionResult.Stdout)

	npmConfigResult, err := executor.Execute("npm", []string{"config", "list"}, 5*time.Second)
	if err == nil && npmConfigResult.ExitCode == 0 {
		result.Metadata["npm_config"] = strings.TrimSpace(npmConfigResult.Stdout)
	}

	globalDirResult, err := executor.Execute("npm", []string{"config", "get", "prefix"}, 5*time.Second)
	if err == nil && globalDirResult.ExitCode == 0 {
		globalDir := strings.TrimSpace(globalDirResult.Stdout)
		result.Metadata["npm_global_dir"] = globalDir

		if _, err := os.Stat(globalDir); err == nil {
			testFile := filepath.Join(globalDir, ".test_write")
			if f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				if closeErr := f.Close(); closeErr != nil {
					// Ignore file close errors during NPM global directory verification test
					_ = closeErr
				}
				if err := os.Remove(testFile); err != nil {
					// Ignore test file removal errors during NPM global directory verification
					_ = err
				}
				result.Metadata["npm_global_writable"] = true
			} else {
				r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryPermissions,
					"NPM Global Directory Not Writable",
					fmt.Sprintf("Cannot write to npm global directory: %s", globalDir),
					"Run: npm config set prefix ~/.npm-global or use sudo for global installs",
					map[string]interface{}{"global_dir": globalDir})
			}
		}
	}
}

func (r *DefaultRuntimeInstaller) verifyNodejsPackageInstallation(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	listResult, err := executor.Execute("npm", []string{"list", "--global", "--depth=0", "--json"}, 10*time.Second)
	if err != nil || listResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
			"NPM List Failed",
			"Unable to list installed npm packages",
			"Check npm installation and permissions",
			map[string]interface{}{"error": err})
		return
	}

	result.Metadata["npm_packages_listable"] = true

	packageCount := strings.Count(listResult.Stdout, "\"version\":")
	result.Metadata["npm_global_packages_count"] = packageCount

	if platform.IsCommandAvailable("npx") {
		result.Metadata["npx_available"] = true
	} else {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryDependencies,
			"NPX Not Available",
			"npx command is not available",
			"Upgrade npm to version 5.2.0 or later to get npx",
			map[string]interface{}{})
	}
}

func (r *DefaultRuntimeInstaller) verifyJavaEnvironment(result *types.VerificationResult) {
	if javaHome := os.Getenv(ENV_VAR_JAVA_HOME); javaHome != "" {
		result.EnvironmentVars[ENV_VAR_JAVA_HOME] = javaHome
		if _, err := os.Stat(javaHome); err != nil {
			r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
				"Invalid JAVA_HOME",
				fmt.Sprintf("JAVA_HOME points to non-existent directory: %s", javaHome),
				"Set JAVA_HOME to a valid JDK installation directory",
				map[string]interface{}{"java_home": javaHome})
		} else {
			jdkPaths := []string{
				filepath.Join(javaHome, "bin", "javac"+platform.GetExecutableExtension()),
				filepath.Join(javaHome, "bin", "java"+platform.GetExecutableExtension()),
			}

			for _, path := range jdkPaths {
				if _, err := os.Stat(path); err != nil {
					r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
						"Incomplete JDK Installation",
						fmt.Sprintf("JAVA_HOME does not contain a complete JDK: missing %s", filepath.Base(path)),
						"Set JAVA_HOME to a complete JDK installation",
						map[string]interface{}{"java_home": javaHome, "missing_file": path})
				}
			}
		}
	} else {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryEnvironment,
			"JAVA_HOME Not Set",
			"JAVA_HOME environment variable is not set",
			"Set JAVA_HOME to your JDK installation directory for better IDE support",
			map[string]interface{}{})
	}

	executor := platform.NewCommandExecutor()
	systemInfoResult, err := executor.Execute("java", []string{"-XshowSettings:properties", "-version"}, 5*time.Second)
	if err == nil {
		output := systemInfoResult.Stderr + systemInfoResult.Stdout
		result.Metadata["java_system_info"] = output

		r.parseJavaSystemInfo(result, output)
	}
}

func (r *DefaultRuntimeInstaller) parseJavaSystemInfo(result *types.VerificationResult, output string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "java.version") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				result.Metadata["java_version_property"] = strings.TrimSpace(parts[1])
			}
		}

		if strings.Contains(line, "java.home") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				result.Metadata["java_home_property"] = strings.TrimSpace(parts[1])
			}
		}

		if strings.Contains(line, "java.vendor") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				result.Metadata["java_vendor"] = strings.TrimSpace(parts[1])
			}
		}
	}
}

func (r *DefaultRuntimeInstaller) verifyJavaCompiler(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	if !platform.IsCommandAvailable("javac") {
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			"Java Compiler Not Available",
			"javac (Java compiler) is not available",
			"Install a JDK (not just JRE) to get the Java compiler",
			map[string]interface{}{})
		return
	}

	javacVersionResult, err := executor.Execute("javac", []string{"-version"}, 5*time.Second)
	if err != nil || javacVersionResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
			"Java Compiler Version Check Failed",
			"javac is available but version check failed",
			RECOMMENDATION_JDK_CORRUPTION,
			map[string]interface{}{"error": err})
		return
	}

	javacVersion := strings.TrimSpace(javacVersionResult.Stderr)
	if javacVersion == "" {
		javacVersion = strings.TrimSpace(javacVersionResult.Stdout)
	}
	result.Metadata["javac_version"] = javacVersion

	if javaVersion, exists := result.Metadata["java_version_property"].(string); exists {
		if !r.versionsMatch(javaVersion, javacVersion) {
			r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryConfiguration,
				"Java Version Mismatch",
				"Java runtime and compiler versions don't match",
				"Ensure java and javac are from the same JDK installation",
				map[string]interface{}{
					"java_version":  javaVersion,
					"javac_version": javacVersion,
				})
		}
	}
}

func (r *DefaultRuntimeInstaller) verifyJavaDevelopmentTools(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	if platform.IsCommandAvailable("jar") {
		result.Metadata["jar_available"] = true
	} else {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryDependencies,
			"JAR Tool Missing",
			"jar tool is not available",
			"Install a complete JDK to get all development tools",
			map[string]interface{}{})
	}

	if platform.IsCommandAvailable("javadoc") {
		result.Metadata["javadoc_available"] = true
	} else {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryDependencies,
			"Javadoc Tool Missing",
			"javadoc tool is not available",
			"Install a complete JDK to get documentation tools",
			map[string]interface{}{})
	}

	r.testJavaCompilation(result, executor)
}

func (r *DefaultRuntimeInstaller) testJavaCompilation(result *types.VerificationResult, executor platform.CommandExecutor) {
	tempDir := platform.GetTempDirectory()
	testDir := filepath.Join(tempDir, fmt.Sprintf("java_test_%d", time.Now().Unix()))

	if err := os.MkdirAll(testDir, 0755); err != nil {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryPermissions,
			"Cannot Create Test Directory",
			"Unable to create temporary directory for compilation test",
			"Check write permissions in temporary directory",
			map[string]interface{}{"temp_dir": tempDir, "error": err.Error()})
		return
	}
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			// Ignore cleanup errors in defer
			_ = err
		}
	}()

	javaFile := filepath.Join(testDir, "Hello.java")
	javaCode := `public class Hello {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}`

	if err := os.WriteFile(javaFile, []byte(javaCode), 0644); err != nil {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryPermissions,
			"Cannot Write Test File",
			"Unable to write test Java file",
			"Check write permissions in temporary directory",
			map[string]interface{}{"test_file": javaFile, "error": err.Error()})
		return
	}

	compileResult, err := executor.Execute("javac", []string{javaFile}, 10*time.Second)
	if err != nil || compileResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Java Compilation Test Failed",
			"Unable to compile a simple Java program",
			"Check JDK installation and environment configuration",
			map[string]interface{}{
				"error":     err,
				"exit_code": compileResult.ExitCode,
				"stderr":    compileResult.Stderr,
			})
		return
	}

	classFile := filepath.Join(testDir, "Hello.class")
	if _, err := os.Stat(classFile); err != nil {
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Java Class File Not Created",
			"Compilation succeeded but class file was not created",
			RECOMMENDATION_JDK_CORRUPTION,
			map[string]interface{}{"class_file": classFile})
		return
	}

	runResult, err := executor.Execute("java", []string{"-cp", testDir, "Hello"}, 5*time.Second)
	if err != nil || runResult.ExitCode != 0 {
		r.addIssue(result, types.IssueSeverityLow, types.IssueCategoryExecution,
			"Java Execution Test Failed",
			"Compiled Java program could not be executed",
			"Check Java runtime environment configuration",
			map[string]interface{}{
				"error":     err,
				"exit_code": runResult.ExitCode,
				"stderr":    runResult.Stderr,
			})
	} else {
		result.Metadata["java_compilation_test"] = "passed"
		result.Metadata["java_execution_test"] = "passed"
	}
}

func (r *DefaultRuntimeInstaller) versionsMatch(version1, version2 string) bool {
	versionRegex := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

	matches1 := versionRegex.FindStringSubmatch(version1)
	matches2 := versionRegex.FindStringSubmatch(version2)

	if len(matches1) < 4 || len(matches2) < 4 {
		return false
	}

	for i := 1; i <= 2; i++ {
		v1, _ := strconv.Atoi(matches1[i])
		v2, _ := strconv.Atoi(matches2[i])
		if v1 != v2 {
			return false
		}
	}

	return true
}
