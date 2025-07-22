package installer

const (
	RuntimeGo     = "go"
	RuntimePython = "python"
	RuntimeNodeJS = "nodejs"
	RuntimeJava   = "java"
)

const (
	ServerGopls                    = "gopls"
	ServerPylsp                    = "pylsp"
	ServerTypeScriptLanguageServer = "typescript-language-server"
	ServerJDTLS                    = "jdtls"
)

const (
	StatusUnknown = "unknown"
	StatusPassed  = "passed"
	VersionLatest = "latest"
	TestName      = "test"
)

const (
	CommandNode      = "node"
	CommandBrew      = "brew"
	CommandCmd       = "cmd"
	CommandEcho      = "echo"
	PathUsrLocalBin  = "/usr/local/bin"
	ExecutableExtWin = ".exe"
)

const (
	PackageManagerHomebrew   = "homebrew"
	PackageManagerApt        = "apt"
	PackageManagerWinget     = "winget"
	PackageManagerChocolatey = "chocolatey"
)

const (
	JavaVendorOracle  = "Oracle JDK"
	JavaVendorOpenJDK = "OpenJDK"
)

const (
	TestVersionGo    = "1.19.0"
	CheckTypeVersion = "version_check"
	RuntimeKeyword   = "runtime"
)

const (
	// Platform constants
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformDarwin  = "darwin"

	// Architecture constants
	ArchAMD64 = "amd64"

	// Version constants
	UbuntuVersion = "20.04"

	// Java JDTLS output for testing
	JavaVersionOutput = `openjdk version "17.0.2" 2022-01-18
OpenJDK Runtime Environment (build 17.0.2+8-Ubuntu-120.04)
OpenJDK 64-Bit Server VM (build 17.0.2+8-Ubuntu-120.04, mixed mode, sharing)`
)

const (
	ENV_VAR_JAVA_HOME = "JAVA_HOME"
	ENV_VAR_PATH      = "PATH"

	RECOMMENDATION_JDK_CORRUPTION    = "Check JDK installation for corruption"
	RECOMMENDATION_GO_INSTALL        = "Install Go from https://golang.org/dl/"
	RECOMMENDATION_WRITE_PERMISSIONS = "Check write permissions in temporary directory"
	RECOMMENDATION_JDK_INSTALL       = "Install a JDK (not just JRE) to get the Java compiler"

	ERROR_UNKNOWN_SERVER          = "unknown server: %s"
	ERROR_WINDOWS_NOT_IMPLEMENTED = "windows server installation not implemented yet"

	LABEL_PACKAGE      = "package"
	LABEL_INSTALLED    = "installed"
	LABEL_POST_INSTALL = "post-install"
	LABEL_SHARE        = "share"
	LABEL_HOME         = "HOME"
	LABEL_PLUGINS      = "plugins"
	LABEL_WHICH        = "which"
	LABEL_NODE_MODULES = "node_modules"
	LABEL_USER_AGENT   = "User-Agent"
	LABEL_CONFIG_LINUX = "config_linux"
	LABEL_JDTLS_BAT    = "jdtls.bat"

	FORMAT_INSTALL_ACTION = "Install %s runtime"
	FORMAT_TEST_WRITE     = ".test_write"
	FORMAT_VERSION_OUTPUT = "Version: %s\n"

	BINARY_LSPGATEWAY = "lsp-gateway"

	// Error messages
	ERROR_MACOS_NOT_IMPLEMENTED = "macOS server installation not implemented yet"
	ERROR_LINUX_NOT_IMPLEMENTED = "linux server installation not implemented yet"

	// Platform strategy formats
	FORMAT_PLATFORM_STRATEGY = "%s_platform_strategy"

	// Java-related constants
	JAVA_HOME_NOT_SET = "JAVA_HOME environment variable is not set"
	JDTLS_JAR_PATTERN = "org.eclipse.jdt.ls.core_*.jar"

	// Homebrew packages
	HOMEBREW_PYTHON_3_11 = "python@3.11"
	HOMEBREW_OPENJDK_21  = "openjdk@21"
)
