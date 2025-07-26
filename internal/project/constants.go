package project

const (
	// Project detection status messages
	PROJECT_DETECTION_STARTED    = "Project detection started"
	PROJECT_DETECTION_COMPLETED  = "Project detection completed"
	PROJECT_DETECTION_FAILED     = "Project detection failed: %w"
	PROJECT_WORKSPACE_ROOT_FOUND = "Project workspace root found"
	PROJECT_TYPE_DETECTED        = "Project type detected: %s"
	PROJECT_VALIDATION_PASSED    = "Project validation passed"
	PROJECT_VALIDATION_FAILED    = "Project validation failed: %v"

	// Multi-language project handling
	MULTI_LANGUAGE_PROJECT_DETECTED = "Multi-language project detected"
	PRIMARY_LANGUAGE_DETERMINED     = "Primary language determined: %s"
	SECONDARY_LANGUAGES_FOUND       = "Secondary languages found: %v"

	// Project configuration messages
	PROJECT_CONFIG_LOADED    = "Project configuration loaded"
	PROJECT_CONFIG_GENERATED = "Project configuration generated"
	PROJECT_CONFIG_INVALID   = "Project configuration is invalid: %v"

	// Language-specific detection
	GO_PROJECT_DETECTED         = "Go project detected"
	PYTHON_PROJECT_DETECTED     = "Python project detected"
	NODEJS_PROJECT_DETECTED     = "Node.js project detected"
	JAVA_PROJECT_DETECTED       = "Java project detected"
	TYPESCRIPT_PROJECT_DETECTED = "TypeScript project detected"
	MIXED_PROJECT_DETECTED      = "Mixed-language project detected"

	// Format strings
	FORMAT_PROJECT_INFO      = "Project: %s (%s) at %s"
	FORMAT_PROJECT_LANGUAGES = "Languages: %v"
	FORMAT_PROJECT_SERVERS   = "Required servers: %v"
	FORMAT_PROJECT_MODULE    = "Module: %s (version: %s)"

	// Log field names
	LOG_FIELD_PROJECT_TYPE     = "project_type"
	LOG_FIELD_PROJECT_ROOT     = "project_root"
	LOG_FIELD_PROJECT_LANGUAGE = "project_language"
	LOG_FIELD_PROJECT_MODULE   = "project_module"
	LOG_FIELD_PROJECT_VERSION  = "project_version"
	LOG_FIELD_DETECTION_TIME   = "detection_time"
	LOG_FIELD_WORKSPACE_ROOT   = "workspace_root"
	LOG_FIELD_SERVERS_REQUIRED = "servers_required"

	// Project types
	PROJECT_TYPE_GO         = "go"
	PROJECT_TYPE_PYTHON     = "python"
	PROJECT_TYPE_NODEJS     = "nodejs"
	PROJECT_TYPE_JAVA       = "java"
	PROJECT_TYPE_TYPESCRIPT = "typescript"
	PROJECT_TYPE_MIXED      = "mixed"
	PROJECT_TYPE_UNKNOWN    = "unknown"

	// Project marker files
	MARKER_GO_MOD       = "go.mod"
	MARKER_GO_SUM       = "go.sum"
	MARKER_PACKAGE_JSON = "package.json"
	MARKER_YARN_LOCK    = "yarn.lock"
	MARKER_PNPM_LOCK    = "pnpm-lock.yaml"
	MARKER_SETUP_PY     = "setup.py"
	MARKER_PYPROJECT    = "pyproject.toml"
	MARKER_REQUIREMENTS = "requirements.txt"
	MARKER_PIPFILE      = "Pipfile"
	MARKER_POM_XML      = "pom.xml"
	MARKER_BUILD_GRADLE = "build.gradle"
	MARKER_TSCONFIG     = "tsconfig.json"
	MARKER_CARGO_TOML   = "Cargo.toml"

	// Directory names that indicate project types
	DIR_NODE_MODULES = "node_modules"
	DIR_SRC          = "src"
	DIR_LIB          = "lib"
	DIR_BUILD        = "build"
	DIR_DIST         = "dist"
	DIR_TARGET       = "target"
	DIR_VENV         = "venv"
	DIR_ENV          = "env"
	DIR_VENDOR       = "vendor"
	DIR_DOT_GIT      = ".git"

	// Server requirements mapping
	SERVER_GOPLS                  = "gopls"
	SERVER_PYLSP                  = "pylsp"
	SERVER_TYPESCRIPT_LANG_SERVER = "typescript-language-server"
	SERVER_JDTLS                  = "jdtls"
	SERVER_RUST_ANALYZER          = "rust-analyzer"

	// Detection priorities (higher number = higher priority)
	PRIORITY_GO         = 100
	PRIORITY_PYTHON     = 90
	PRIORITY_TYPESCRIPT = 85
	PRIORITY_NODEJS     = 80
	PRIORITY_JAVA       = 70
	PRIORITY_MIXED      = 50
	PRIORITY_UNKNOWN    = 0

	// Validation messages
	WORKSPACE_ROOT_NOT_FOUND     = "Workspace root not found"
	PROJECT_MARKERS_NOT_FOUND    = "No project markers found"
	INVALID_PROJECT_STRUCTURE    = "Invalid project structure"
	MULTIPLE_PROJECT_ROOTS_FOUND = "Multiple project roots found"

	// Error recovery messages
	ATTEMPTING_FALLBACK_DETECTION = "Attempting fallback detection"
	USING_DIRECTORY_AS_ROOT       = "Using directory as project root"
	INFERRING_PROJECT_TYPE        = "Inferring project type from directory structure"

	// Timeout and performance
	DEFAULT_DETECTION_TIMEOUT = "30s"
	MAX_DIRECTORY_DEPTH       = 10
	MAX_FILES_TO_SCAN         = 1000

	// Project status
	PROJECT_STATUS_VALID      = "valid"
	PROJECT_STATUS_INVALID    = "invalid"
	PROJECT_STATUS_INCOMPLETE = "incomplete"
	PROJECT_STATUS_CORRUPTED  = "corrupted"
)
