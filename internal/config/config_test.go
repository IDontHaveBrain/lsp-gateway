package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *GatewayConfig
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Port zero (gets defaulted)",
			config: &GatewayConfig{
				Port: 0,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: false, // Port 0 is valid and gets defaulted
		},
		{
			name: "Invalid port - negative",
			config: &GatewayConfig{
				Port: -1,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid port - too large",
			config: &GatewayConfig{
				Port: 65536,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No servers",
			config: &GatewayConfig{
				Port:    8080,
				Servers: []ServerConfig{},
			},
			wantErr: true,
		},
		{
			name: "Duplicate server names",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
					{
						Name:      "go-lsp",
						Languages: []string{"python"},
						Command:   "pylsp",
						Transport: "stdio",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid server configuration",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{
						Name:      "",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GatewayConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name: "Valid server configuration",
			config: &ServerConfig{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			wantErr: false,
		},
		{
			name: "Empty name",
			config: &ServerConfig{
				Name:      "",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "No languages",
			config: &ServerConfig{
				Name:      "go-lsp",
				Languages: []string{},
				Command:   "gopls",
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "Empty command",
			config: &ServerConfig{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "",
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "Empty transport",
			config: &ServerConfig{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "",
			},
			wantErr: true,
		},
		{
			name: "Invalid transport",
			config: &ServerConfig{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Valid TCP transport",
			config: &ServerConfig{
				Name:      "tcp-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "tcp",
			},
			wantErr: false,
		},
		{
			name: "Valid HTTP transport",
			config: &ServerConfig{
				Name:      "http-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "http",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Port)
	}

	if len(config.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(config.Servers))
	}

	server := config.Servers[0]
	if server.Name != "go-lsp" {
		t.Errorf("Expected server name 'go-lsp', got '%s'", server.Name)
	}

	if len(server.Languages) != 1 || server.Languages[0] != "go" {
		t.Errorf("Expected language 'go', got %v", server.Languages)
	}

	if server.Command != "gopls" {
		t.Errorf("Expected command 'gopls', got '%s'", server.Command)
	}

	if server.Transport != "stdio" {
		t.Errorf("Expected transport 'stdio', got '%s'", server.Transport)
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestGetServerByLanguage(t *testing.T) {
	t.Parallel()
	config := &GatewayConfig{
		Port: 8080,
		Servers: []ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			{
				Name:      "python-lsp",
				Languages: []string{"python"},
				Command:   "pylsp",
				Transport: "stdio",
			},
			{
				Name:      "multi-lsp",
				Languages: []string{"typescript", "javascript"},
				Command:   "ts-lsp",
				Transport: "stdio",
			},
		},
	}

	tests := []struct {
		language     string
		expectedName string
		shouldExist  bool
	}{
		{"go", "go-lsp", true},
		{"python", "python-lsp", true},
		{"typescript", "multi-lsp", true},
		{"javascript", "multi-lsp", true},
		{"rust", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			server, err := config.GetServerByLanguage(tt.language)

			if tt.shouldExist {
				if err != nil {
					t.Errorf("Expected server for language '%s', got error: %v", tt.language, err)
				}
				if server == nil {
					t.Errorf("Expected server for language '%s', got nil", tt.language)
				} else if server.Name != tt.expectedName {
					t.Errorf("Expected server name '%s', got '%s'", tt.expectedName, server.Name)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for language '%s', got none", tt.language)
				}
				if server != nil {
					t.Errorf("Expected nil server for language '%s', got %v", tt.language, server)
				}
			}
		})
	}
}

func TestGetServerByName(t *testing.T) {
	t.Parallel()
	config := &GatewayConfig{
		Port: 8080,
		Servers: []ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			{
				Name:      "python-lsp",
				Languages: []string{"python"},
				Command:   "pylsp",
				Transport: "stdio",
			},
		},
	}

	tests := []struct {
		name        string
		shouldExist bool
	}{
		{"go-lsp", true},
		{"python-lsp", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := config.GetServerByName(tt.name)

			if tt.shouldExist {
				if err != nil {
					t.Errorf("Expected server with name '%s', got error: %v", tt.name, err)
				}
				if server == nil {
					t.Errorf("Expected server with name '%s', got nil", tt.name)
				} else if server.Name != tt.name {
					t.Errorf("Expected server name '%s', got '%s'", tt.name, server.Name)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for name '%s', got none", tt.name)
				}
				if server != nil {
					t.Errorf("Expected nil server for name '%s', got %v", tt.name, server)
				}
			}
		})
	}
}

func createTempConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	return configFile
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	validConfig := `
port: 8080
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio
  - name: python-lsp
    languages: [python]
    command: pylsp
    transport: stdio
`

	invalidYAML := `
port: 8080
servers:
  - name: go-lsp
    languages: [go
    command: gopls
    transport: stdio
`

	configWithDefaults := `
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
`

	tests := []struct {
		name              string
		configPath        string
		configData        string
		wantErr           bool
		expectedPort      int
		expectedTransport string
	}{
		{
			name:              "Valid configuration",
			configData:        validConfig,
			wantErr:           false,
			expectedPort:      8080,
			expectedTransport: "stdio",
		},
		{
			name:       "Invalid YAML",
			configData: invalidYAML,
			wantErr:    true,
		},
		{
			name:              "Configuration with defaults",
			configData:        configWithDefaults,
			wantErr:           false,
			expectedPort:      8080,
			expectedTransport: "stdio",
		},
		{
			name:       "Nonexistent file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "Empty config path (should use config.yaml)",
			configPath: "",
			wantErr:    true, // because default config.yaml doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string

			if tt.configData != "" {
				configPath = createTempConfigFile(t, tt.configData)
			} else {
				configPath = tt.configPath
			}

			config, err := LoadConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Error("Expected config but got nil")
				return
			}

			if config.Port != tt.expectedPort {
				t.Errorf("Expected port %d, got %d", tt.expectedPort, config.Port)
			}

			if len(config.Servers) > 0 && config.Servers[0].Transport != tt.expectedTransport {
				t.Errorf("Expected transport %s, got %s", tt.expectedTransport, config.Servers[0].Transport)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *GatewayConfig
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Nil configuration",
			config:  nil,
			wantErr: true,
		},
		{
			name: "Invalid configuration - no servers",
			config: &GatewayConfig{
				Port:    0, // Port 0 is valid, but no servers is invalid
				Servers: []ServerConfig{},
			},
			wantErr: true, // Fails because no servers configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadAndValidateConfig(t *testing.T) {
	t.Parallel()
	validConfig := `
port: 8080
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio
`

	invalidConfig := `
port: 0
servers: []
`

	tests := []struct {
		name       string
		configData string
		wantErr    bool
	}{
		{
			name:       "Valid configuration",
			configData: validConfig,
			wantErr:    false,
		},
		{
			name:       "Invalid configuration",
			configData: invalidConfig,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := createTempConfigFile(t, tt.configData)

			config, err := LoadConfig(configPath)
			if err == nil {
				err = ValidateConfig(config)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Error("Expected config but got nil")
			}
		})
	}
}

func TestLoadConfigComplexConfiguration(t *testing.T) {
	t.Parallel()
	complexConfig := `
port: 9090
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    args: ["-rpc.trace", "-logfile", "/tmp/gopls.log"]
    transport: stdio
  - name: python-lsp
    languages: [python]
    command: python
    args: ["-m", "pylsp"]
    transport: stdio
  - name: typescript-lsp
    languages: [typescript, javascript]
    command: typescript-language-server
    args: ["--stdio"]
    transport: stdio
  - name: rust-lsp
    languages: [rust]
    command: rust-analyzer
    args: []
    transport: stdio
`

	configPath := createTempConfigFile(t, complexConfig)

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load complex config: %v", err)
	}

	if config.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Port)
	}

	if len(config.Servers) != 4 {
		t.Errorf("Expected 4 servers, got %d", len(config.Servers))
	}

	goServer := config.Servers[0]
	if goServer.Name != "go-lsp" {
		t.Errorf("Expected first server name 'go-lsp', got '%s'", goServer.Name)
	}

	if len(goServer.Args) != 3 {
		t.Errorf("Expected 3 args for go-lsp, got %d", len(goServer.Args))
	}

	tsServer := config.Servers[2]
	if len(tsServer.Languages) != 2 {
		t.Errorf("Expected 2 languages for typescript-lsp, got %d", len(tsServer.Languages))
	}

	if err := ValidateConfig(config); err != nil {
		t.Errorf("Complex config should be valid: %v", err)
	}
}
