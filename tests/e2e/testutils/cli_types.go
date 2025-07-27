package testutils

// CLI test types shared between setup_cli_e2e_test.go and npm_cli_e2e_test.go

// SetupResult represents the JSON output structure from setup commands
type SetupResult struct {
	Success               bool                               `json:"success"`
	Duration              int64                              `json:"duration"`
	RuntimesDetected      map[string]RuntimeDetectionResult `json:"runtimes_detected"`
	ServersInstalled      map[string]ServerInstallResult    `json:"servers_installed,omitempty"`
	ConfigGeneration      *ConfigGenerationResult           `json:"config_generation,omitempty"`
	ProjectDetection      *ProjectDetectionResult           `json:"project_detection,omitempty"`
	ValidationResults     map[string]ValidationResult       `json:"validation_results,omitempty"`
	Errors                []string                           `json:"errors,omitempty"`
	Warnings              []string                           `json:"warnings,omitempty"`
	Summary               *SetupSummary                     `json:"summary"`
	StatusMessages        []string                           `json:"status_messages,omitempty"`
}

type RuntimeDetectionResult struct {
	Name           string                 `json:"Name"`
	Installed      bool                   `json:"Installed"`
	Version        string                 `json:"Version"`
	ParsedVersion  interface{}            `json:"ParsedVersion"`
	Compatible     bool                   `json:"Compatible"`
	MinVersion     string                 `json:"MinVersion"`
	Path           string                 `json:"Path"`
	WorkingDir     string                 `json:"WorkingDir"`
	DetectionCmd   string                 `json:"DetectionCmd"`
	Issues         []string               `json:"Issues"`
	Warnings       []string               `json:"Warnings"`
	Metadata       map[string]interface{} `json:"Metadata"`
	DetectedAt     string                 `json:"DetectedAt"`
	Duration       int64                  `json:"Duration"`
}

type ServerInstallResult struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Error     string `json:"error,omitempty"`
}

type ConfigGenerationResult struct {
	Generated    bool   `json:"generated"`
	Path         string `json:"path"`
	TemplateUsed string `json:"template_used,omitempty"`
	Error        string `json:"error,omitempty"`
}

type ProjectDetectionResult struct {
	ProjectType     string              `json:"project_type"`
	Languages       []string            `json:"languages"`
	Frameworks      []string            `json:"frameworks"`
	BuildSystems    []string            `json:"build_systems"`
	Confidence      float64             `json:"confidence"`
	Recommendations map[string]string   `json:"recommendations"`
}

type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type SetupSummary struct {
	TotalRuntimes        int `json:"total_runtimes"`
	RuntimesInstalled    int `json:"runtimes_installed"`
	RuntimesAlreadyExist int `json:"runtimes_already_exist"`
	RuntimesFailed       int `json:"runtimes_failed"`
	TotalServers         int `json:"total_servers"`
	ServersInstalled     int `json:"servers_installed"`
	ServersAlreadyExist  int `json:"servers_already_exist"`
	ServersFailed        int `json:"servers_failed"`
}