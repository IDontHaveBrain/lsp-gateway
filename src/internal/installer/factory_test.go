package installer

import (
	"testing"

	"lsp-gateway/src/config"
)

func TestCreateInstallManager(t *testing.T) {
	manager := CreateInstallManager()

	if manager == nil {
		t.Fatal("CreateInstallManager() returned nil")
	}

	supportedLanguages := manager.GetSupportedLanguages()
	if len(supportedLanguages) == 0 {
		t.Error("CreateInstallManager() created manager with no installers")
	}

	expectedLanguageCount := 6
	if len(supportedLanguages) != expectedLanguageCount {
		t.Errorf("Expected %d supported languages, got %d", expectedLanguageCount, len(supportedLanguages))
	}

	expectedLanguages := []string{"go", "python", "typescript", "javascript", "java", "rust"}
	languageMap := make(map[string]bool)
	for _, lang := range supportedLanguages {
		languageMap[lang] = true
	}

	for _, expected := range expectedLanguages {
		if !languageMap[expected] {
			t.Errorf("Expected language '%s' not found in supported languages", expected)
		}
	}
}

func TestRegisterAllInstallers(t *testing.T) {
	manager := NewLSPInstallManager()
	platform := NewLSPPlatformInfo()

	registerAllInstallers(manager, platform)

	supportedLanguages := manager.GetSupportedLanguages()
	expectedCount := 6
	if len(supportedLanguages) != expectedCount {
		t.Errorf("Expected %d installers registered, got %d", expectedCount, len(supportedLanguages))
	}

	tests := []struct {
		name     string
		language string
		wantErr  bool
	}{
		{
			name:     "go installer registered",
			language: "go",
			wantErr:  false,
		},
		{
			name:     "python installer registered",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "typescript installer registered",
			language: "typescript",
			wantErr:  false,
		},
		{
			name:     "javascript installer registered",
			language: "javascript",
			wantErr:  false,
		},
		{
			name:     "java installer registered",
			language: "java",
			wantErr:  false,
		},
		{
			name:     "rust installer registered",
			language: "rust",
			wantErr:  false,
		},
		{
			name:     "unregistered language",
			language: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := manager.GetInstaller(tt.language)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error for unregistered language")
				}
				if installer != nil {
					t.Error("Expected nil installer for unregistered language")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error getting installer for %s: %v", tt.language, err)
				}
				if installer == nil {
					t.Errorf("Expected installer for %s, got nil", tt.language)
				}
				// JavaScript installer returns "typescript" since it's the same installer
				expectedLanguage := tt.language
				if tt.language == "javascript" {
					expectedLanguage = "typescript"
				}
				if installer.GetLanguage() != expectedLanguage {
					t.Errorf("Installer language = %v, want %v", installer.GetLanguage(), expectedLanguage)
				}
			}
		})
	}
}

func TestRegisterAllInstallers_SharedTypeScriptInstaller(t *testing.T) {
	manager := NewLSPInstallManager()
	platform := NewLSPPlatformInfo()

	registerAllInstallers(manager, platform)

	typescriptInstaller, err := manager.GetInstaller("typescript")
	if err != nil {
		t.Fatalf("Failed to get typescript installer: %v", err)
	}

	javascriptInstaller, err := manager.GetInstaller("javascript")
	if err != nil {
		t.Fatalf("Failed to get javascript installer: %v", err)
	}

	if typescriptInstaller == javascriptInstaller {
		t.Log("TypeScript and JavaScript share the same installer instance (expected)")
	} else {
		t.Error("TypeScript and JavaScript installers should be the same instance")
	}
}

func TestGetDefaultInstallManager(t *testing.T) {
	manager := GetDefaultInstallManager()

	if manager == nil {
		t.Fatal("GetDefaultInstallManager() returned nil")
	}

	supportedLanguages := manager.GetSupportedLanguages()
	expectedCount := 6
	if len(supportedLanguages) != expectedCount {
		t.Errorf("Expected %d supported languages, got %d", expectedCount, len(supportedLanguages))
	}

	testLanguages := []string{"go", "python", "typescript", "javascript", "java", "rust"}
	for _, lang := range testLanguages {
		installer, err := manager.GetInstaller(lang)
		if err != nil {
			t.Errorf("Default manager missing installer for %s: %v", lang, err)
		}
		if installer == nil {
			t.Errorf("Default manager returned nil installer for %s", lang)
		}
	}
}

func TestFactoryCreateSimpleInstaller(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name        string
		language    string
		command     string
		args        []string
		expectValid bool
	}{
		{
			name:        "valid go installer",
			language:    "go",
			command:     "gopls",
			args:        []string{"serve"},
			expectValid: true,
		},
		{
			name:        "valid python installer",
			language:    "python",
			command:     "jedi-language-server",
			args:        []string{},
			expectValid: true,
		},
		{
			name:        "empty command",
			language:    "test",
			command:     "",
			args:        []string{},
			expectValid: true,
		},
		{
			name:        "custom installer",
			language:    "custom",
			command:     "custom-lsp",
			args:        []string{"--port", "8080"},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer := CreateSimpleInstaller(tt.language, tt.command, tt.args, platform)

			if !tt.expectValid {
				if installer != nil {
					t.Error("Expected nil installer for invalid input")
				}
				return
			}

			if installer == nil {
				t.Fatal("CreateSimpleInstaller() returned nil")
			}

			if installer.GetLanguage() != tt.language {
				t.Errorf("Language = %v, want %v", installer.GetLanguage(), tt.language)
			}

			serverConfig := installer.GetServerConfig()
			if serverConfig == nil {
				t.Fatal("Server config is nil")
			}

			if serverConfig.Command != tt.command {
				t.Errorf("Command = %v, want %v", serverConfig.Command, tt.command)
			}

			if len(serverConfig.Args) != len(tt.args) {
				t.Errorf("Args length = %v, want %v", len(serverConfig.Args), len(tt.args))
			}

			for i, arg := range tt.args {
				if i < len(serverConfig.Args) && serverConfig.Args[i] != arg {
					t.Errorf("Args[%d] = %v, want %v", i, serverConfig.Args[i], arg)
				}
			}
		})
	}
}

func TestFactoryConsistency(t *testing.T) {
	manager1 := CreateInstallManager()
	manager2 := GetDefaultInstallManager()

	languages1 := manager1.GetSupportedLanguages()
	languages2 := manager2.GetSupportedLanguages()

	if len(languages1) != len(languages2) {
		t.Errorf("Inconsistent language count: CreateInstallManager=%d, GetDefaultInstallManager=%d",
			len(languages1), len(languages2))
	}

	languageMap1 := make(map[string]bool)
	for _, lang := range languages1 {
		languageMap1[lang] = true
	}

	for _, lang := range languages2 {
		if !languageMap1[lang] {
			t.Errorf("Language '%s' in GetDefaultInstallManager but not in CreateInstallManager", lang)
		}
	}
}

func TestFactoryPlatformHandling(t *testing.T) {
	platform := NewLSPPlatformInfo()

	if platform.GetPlatform() == "" {
		t.Error("Platform should not be empty")
	}

	if platform.GetArch() == "" {
		t.Error("Architecture should not be empty")
	}

	manager := CreateInstallManager()
	if manager == nil {
		t.Error("Manager creation should succeed regardless of platform")
	}

	supportedLanguages := manager.GetSupportedLanguages()
	if len(supportedLanguages) == 0 {
		t.Error("Should have installers registered even on unsupported platforms")
	}
}

func TestFactoryWithCustomConfig(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name     string
		config   *config.ServerConfig
		language string
	}{
		{
			name: "go with custom args",
			config: &config.ServerConfig{
				Command: "gopls",
				Args:    []string{"serve", "-debug", ":8080"},
			},
			language: "go",
		},
		{
			name: "python with verbose",
			config: &config.ServerConfig{
				Command: "jedi-language-server",
				Args:    []string{"-v"},
			},
			language: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer := NewBaseInstaller(tt.language, tt.config, platform)
			if installer == nil {
				t.Fatal("NewBaseInstaller returned nil")
			}

			if installer.GetLanguage() != tt.language {
				t.Errorf("Language = %v, want %v", installer.GetLanguage(), tt.language)
			}

			serverConfig := installer.GetServerConfig()
			if serverConfig.Command != tt.config.Command {
				t.Errorf("Command = %v, want %v", serverConfig.Command, tt.config.Command)
			}

			if len(serverConfig.Args) != len(tt.config.Args) {
				t.Errorf("Args count = %v, want %v", len(serverConfig.Args), len(tt.config.Args))
			}
		})
	}
}

func TestFactoryErrorScenarios(t *testing.T) {
	manager := NewLSPInstallManager()
	platform := NewLSPPlatformInfo()

	installer, err := manager.GetInstaller("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent language")
	}
	if installer != nil {
		t.Error("Expected nil installer for nonexistent language")
	}

	registerAllInstallers(manager, platform)

	installer, err = manager.GetInstaller("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent language even after registration")
	}
}

func TestFactoryLanguageMapping(t *testing.T) {
	manager := CreateInstallManager()

	languageMappings := map[string]string{
		"go":         "go",
		"python":     "python",
		"typescript": "typescript",
		"javascript": "typescript",
		"java":       "java",
		"rust":       "rust",
	}

	for requestedLang, expectedInstallerLang := range languageMappings {
		installer, err := manager.GetInstaller(requestedLang)
		if err != nil {
			t.Errorf("Failed to get installer for %s: %v", requestedLang, err)
			continue
		}

		if requestedLang == "javascript" {
			if installer.GetLanguage() != "typescript" && installer.GetLanguage() != "javascript" {
				t.Errorf("JavaScript installer should handle TypeScript language, got %s", installer.GetLanguage())
			}
		} else {
			if installer.GetLanguage() != expectedInstallerLang {
				t.Errorf("Installer for %s has language %s, want %s", requestedLang, installer.GetLanguage(), expectedInstallerLang)
			}
		}
	}
}

func TestFactoryRegistrationCompleteness(t *testing.T) {
	manager := NewLSPInstallManager()
	platform := NewLSPPlatformInfo()

	if len(manager.GetSupportedLanguages()) != 0 {
		t.Error("New manager should have no installers initially")
	}

	registerAllInstallers(manager, platform)

	supportedLanguages := manager.GetSupportedLanguages()
	expectedLanguages := []string{"go", "python", "typescript", "javascript", "java", "rust"}

	if len(supportedLanguages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages after registration, got %d", len(expectedLanguages), len(supportedLanguages))
	}

	for _, expectedLang := range expectedLanguages {
		found := false
		for _, supportedLang := range supportedLanguages {
			if supportedLang == expectedLang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language '%s' not found in supported languages", expectedLang)
		}
	}

	status := manager.GetStatus()
	for _, expectedLang := range expectedLanguages {
		if langStatus, exists := status[expectedLang]; !exists {
			t.Errorf("Status missing for language '%s'", expectedLang)
		} else {
			if !langStatus.Available {
				t.Errorf("Language '%s' should be marked as available", expectedLang)
			}
			if langStatus.Language != expectedLang {
				t.Errorf("Status language field = %v, want %v", langStatus.Language, expectedLang)
			}
		}
	}
}
