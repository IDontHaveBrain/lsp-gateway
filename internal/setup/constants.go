package setup

const (
	CONFIG_GENERATED_INVALID    = "Generated configuration is invalid: %v"
	CONFIG_VALIDATION_FAILED    = "configuration validation failed: %w"
	CONFIG_YAML_FILE            = "config.yaml"
	RUNTIME_DETECTION_FAILED    = "runtime detection failed: %w"
	RUNTIME_DETECTION_COMPLETED = "Runtime detection completed"
	RUNTIME_WORD                = "runtime"

	DRY_RUN_INSTALL  = "DRY RUN: Would install %s"
	WIZARD_SEPARATOR = "================"

	FORMAT_STARTING               = "Starting %s"
	FORMAT_COMPLETED              = "Completed %s"
	FORMAT_SUCCESSFULLY_INSTALLED = "Successfully installed %s"
	FORMAT_VERSION_FORMAT         = "%d.%d.%d"
	FORMAT_ERROR_TYPE             = "%s error for %s: %s"
	FORMAT_STATUS_VALUE           = "%s: %v"
	FORMAT_OPERATION_STAGE        = "(%s)"

	VERIFICATION_FAILED        = "Verification failed: %v"
	SERVER_VERIFICATION_FAILED = "Server verification failed"
	SERVER_FUNCTIONAL_WARNING  = "Server %s may not be functional"
	CONFIG_GENERATION_FAILED   = "Failed to generate config for %s: %v"
	UNSUPPORTED_RUNTIME_ERROR  = "unsupported runtime: %s"
	INVALID_VERSION_FORMAT     = "invalid version format: %s (cleaned: %s)"
	INVALID_PATCH_VERSION      = "invalid patch version '%s': %v"

	LOG_FIELD_WARNINGS_ISSUED    = "warnings_issued"
	LOG_FIELD_ERRORS_ENCOUNTERED = "errors_encountered"
	LOG_FIELD_OPERATION          = "operation"
	LOG_FIELD_STAGE              = "stage"
	LOG_FIELD_ERROR_CODE         = "error_code"

	PATH_USR_LOCAL         = "/usr/local"
	PATH_USR_BIN_PYTHON    = "/usr/bin/python"
	PATH_HOME_ENV          = "HOME"
	PATH_SETUP_LSP_SERVERS = "setup-lsp-servers"

	VERSION_FLAG = "--version"
	VERSION_WORD = "version"

	COMMAND_AVAILABILITY = "command_availability"
	COMMAND_WORD         = "command"
	COMMAND_LIST         = "list"
	COMMAND_UPDATE       = "update"
	COMMAND_PYTHON       = "python"
	COMMAND_JAVAC        = "javac"

	PACKAGE_MANAGER_DNF  = "dnf"
	PACKAGE_MANAGER_NPM  = "npm"
	PACKAGE_MANAGER_PNPM = "pnpm"

	RUNTIME_PYTHON = "python"
	RUNTIME_NODE   = "node"
	RUNTIME_JAVA   = "java"
	RUNTIME_GO     = "go"

	CURL_COMMAND           = "curl"
	TYPESCRIPT_LANG_SERVER = "typescript-language-server"

	UNKNOWN_DISPLAY_NAME = "Unknown"
	ORACLE_JDK           = "Oracle JDK"
	YARN_COMMAND         = "yarn"

	DOWNLOAD_GO_BINARY = "Downloading Go binary"
	DOWNLOAD_ACTION    = "download"

	INVALID_MAJOR_VERSION = "invalid major version '%s': %v"

	EXAMPLE_BINARY     = "lsp-gateway"
	JAVA_EXE_EXTENSION = ".exe"
	NODE_MODULES_DIR   = "node_modules"
	GOROOT_ENV_VAR     = "GOROOT"
	PYTHONPATH_ENV_VAR = "PYTHONPATH"

	CHECK_WRITE_PERMISSIONS      = "Check write permissions in temporary directory"
	ECLIPSE_JDT_LS               = "eclipse.jdt.ls"
	ORG_ECLIPSE_EQUINOX_LAUNCHER = "org.eclipse.equinox.launcher"
	OPENJDK_21                   = "openjdk@21"
	PYTHON_3_11                  = "python@3.11"
	POST_INSTALL                 = "post-install"
	INSTALLED_STATUS             = "installed"

	// Language servers
	SERVER_GOPLS = "gopls"
	SERVER_PYLSP = "pylsp"
	SERVER_JDTLS = "jdtls"

	// Validation and error messages
	VALIDATION_FAILED_FOR = "Validation failed for %s"
	OPERATION_WORD        = "operation"

	// Installation actions
	INSTALL_ACTION            = "install"
	VERIFYING_GO_INSTALLATION = "Verifying Go installation"

	// File system paths and directories
	WHICH_COMMAND = "which"
	JAR_EXTENSION = "jar"
	PACKAGE_WORD  = "package"

	// Version checking
	VERSION_CHECK_CMD = "version_check"

	// Commands and executables
	COMMAND_WGET = "wget"
	COMMAND_CURL = "curl"

	// Environment variables
	JAVA_HOME_ENV     = "JAVA_HOME"
	JAVA_HOME_NOT_SET = "JAVA_HOME environment variable is not set"

	// Installation types and managers
	HOMEBREW_MANAGER = "homebrew"

	// Logging fields
	LOG_FIELD_COMMANDS_EXECUTED   = "commands_executed"
	LOG_FIELD_WARNINGS_ISSUED_ALT = "warnings_issued"

	// Server verification
	SERVER_NOT_INSTALLED = "Server not installed"

	// Error types
	ERROR_TYPE_FORMAT = "%s error for %s: %s (caused by: %v)"

	// Progress and status
	PROGRESS_UNKNOWN = "unknown"
)
