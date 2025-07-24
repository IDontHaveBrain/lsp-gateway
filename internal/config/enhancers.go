package config

import (
	"fmt"
	"strings"
)

// Framework Enhancers

type ReactEnhancer struct{}

func (r *ReactEnhancer) EnhanceConfig(config *ServerConfig, framework *Framework) error {
	// Enhance TypeScript/JavaScript settings for React
	for _, lang := range config.Languages {
		if lang == "typescript" || lang == "javascript" {
			if settings, ok := config.Settings[lang].(map[string]interface{}); ok {
				if preferences, ok := settings["preferences"].(map[string]interface{}); ok {
					preferences["jsx"] = "react-jsx"
					preferences["jsxAttributeCompletionStyle"] = "auto"
				}
				
				// Add React-specific completions
				if suggest, ok := settings["suggest"].(map[string]interface{}); ok {
					suggest["autoImports"] = true
					suggest["completeJSDocs"] = true
				}
			}
		}
	}
	
	// Add React-specific dependencies
	config.Dependencies = append(config.Dependencies, "typescript", "@types/react", "@types/react-dom")
	
	// Add React framework to server config
	if !contains(config.Frameworks, "react") {
		config.Frameworks = append(config.Frameworks, "react")
	}
	
	return nil
}

func (r *ReactEnhancer) GetRequiredSettings() map[string]interface{} {
	return map[string]interface{}{
		"jsx": "react-jsx",
		"jsxAttributeCompletionStyle": "auto",
	}
}

func (r *ReactEnhancer) GetRecommendedExtensions() []string {
	return []string{"typescript", "javascript", "jsx", "tsx"}
}

type DjangoEnhancer struct{}

func (d *DjangoEnhancer) EnhanceConfig(config *ServerConfig, framework *Framework) error {
	// Enhance Python settings for Django
	if pylsp, ok := config.Settings["pylsp"].(map[string]interface{}); ok {
		if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
			plugins["pylsp_django"] = map[string]bool{"enabled": true}
			plugins["django-stubs"] = map[string]bool{"enabled": true}
			
			// Enable Django-specific rope completion
			if rope, ok := plugins["rope_completion"].(map[string]interface{}); ok {
				rope["enabled"] = true
				rope["eager"] = true
			}
		}
		
		// Add Django-specific configuration sources
		if configSources, ok := pylsp["configurationSources"].([]string); ok {
			pylsp["configurationSources"] = append(configSources, "django", "django-stubs")
		}
	}
	
	// Add Django-specific root markers
	djangoMarkers := []string{"manage.py", "django_project", "settings.py"}
	for _, marker := range djangoMarkers {
		if !contains(config.RootMarkers, marker) {
			config.RootMarkers = append(config.RootMarkers, marker)
		}
	}
	
	// Add Django framework to server config
	if !contains(config.Frameworks, "django") {
		config.Frameworks = append(config.Frameworks, "django")
	}
	
	return nil
}

func (d *DjangoEnhancer) GetRequiredSettings() map[string]interface{} {
	return map[string]interface{}{
		"pylsp_django": map[string]bool{"enabled": true},
		"django-stubs": map[string]bool{"enabled": true},
	}
}

func (d *DjangoEnhancer) GetRecommendedExtensions() []string {
	return []string{"python", "django", "html", "css"}
}

type SpringBootEnhancer struct{}

func (s *SpringBootEnhancer) EnhanceConfig(config *ServerConfig, framework *Framework) error {
	// Enhance Java settings for Spring Boot
	if java, ok := config.Settings["java"].(map[string]interface{}); ok {
		if completion, ok := java["completion"].(map[string]interface{}); ok {
			if favoriteStatic, ok := completion["favoriteStaticMembers"].([]string); ok {
				springImports := []string{
					"org.springframework.boot.test.context.SpringBootTest.*",
					"org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*",
					"org.springframework.test.web.servlet.result.MockMvcResultMatchers.*",
					"org.springframework.boot.autoconfigure.SpringBootApplication.*",
					"org.springframework.web.bind.annotation.*",
					"org.springframework.stereotype.*",
				}
				
				// Add Spring Boot imports if not already present
				for _, springImport := range springImports {
					if !contains(favoriteStatic, springImport) {
						favoriteStatic = append(favoriteStatic, springImport)
					}
				}
				completion["favoriteStaticMembers"] = favoriteStatic
			}
		}
		
		// Add Spring Boot specific configuration
		if configuration, ok := java["configuration"].(map[string]interface{}); ok {
			if runtimes, ok := configuration["runtimes"].([]map[string]interface{}); ok {
				// Ensure Spring Boot compatible Java versions are available
				hasJava11 := false
				hasJava17 := false
				
				for _, runtime := range runtimes {
					if name, ok := runtime["name"].(string); ok {
						if strings.Contains(name, "11") {
							hasJava11 = true
						}
						if strings.Contains(name, "17") {
							hasJava17 = true
						}
					}
				}
				
				if !hasJava11 {
					runtimes = append(runtimes, map[string]interface{}{
						"name": "JavaSE-11",
						"path": "/usr/lib/jvm/java-11-openjdk",
					})
				}
				
				if !hasJava17 {
					runtimes = append(runtimes, map[string]interface{}{
						"name": "JavaSE-17",
						"path": "/usr/lib/jvm/java-17-openjdk",
					})
				}
				
				configuration["runtimes"] = runtimes
			}
		}
	}
	
	// Add Spring Boot specific root markers
	springMarkers := []string{"pom.xml", "build.gradle", "application.properties", "application.yml"}
	for _, marker := range springMarkers {
		if !contains(config.RootMarkers, marker) {
			config.RootMarkers = append(config.RootMarkers, marker)
		}
	}
	
	// Add Spring Boot framework to server config
	if !contains(config.Frameworks, "spring-boot") {
		config.Frameworks = append(config.Frameworks, "spring-boot")
	}
	
	return nil
}

func (s *SpringBootEnhancer) GetRequiredSettings() map[string]interface{} {
	return map[string]interface{}{
		"favoriteStaticMembers": []string{
			"org.springframework.boot.test.context.SpringBootTest.*",
			"org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*",
			"org.springframework.test.web.servlet.result.MockMvcResultMatchers.*",
		},
	}
}

func (s *SpringBootEnhancer) GetRecommendedExtensions() []string {
	return []string{"java", "xml", "yaml", "properties"}
}

type KubernetesEnhancer struct{}

func (k *KubernetesEnhancer) EnhanceConfig(config *ServerConfig, framework *Framework) error {
	// Enhance settings for Kubernetes projects
	for _, lang := range config.Languages {
		switch lang {
		case "go":
			if gopls, ok := config.Settings["gopls"].(map[string]interface{}); ok {
				// Add Kubernetes-specific build flags
				gopls["buildFlags"] = []string{"-tags=ignore_autogenerated"}
				
				// Enhance analyses for Kubernetes code patterns
				if analyses, ok := gopls["analyses"].(map[string]bool); ok {
					analyses["deepequalerrors"] = true
					analyses["fieldalignment"] = false // Often problematic with generated code
					analyses["unusedwrite"] = false   // Common in controller patterns
				}
			}
		case "yaml":
			// Add YAML language server configuration for Kubernetes
			config.Settings["yaml"] = map[string]interface{}{
				"schemas": map[string]interface{}{
					"kubernetes": "*.yaml",
					"kustomize":  "kustomization.yaml",
				},
				"validate": true,
				"completion": true,
				"hover": true,
			}
		}
	}
	
	// Add Kubernetes-specific root markers
	k8sMarkers := []string{"kustomization.yaml", "Chart.yaml", "deployment.yaml", "service.yaml", "Dockerfile"}
	for _, marker := range k8sMarkers {
		if !contains(config.RootMarkers, marker) {
			config.RootMarkers = append(config.RootMarkers, marker)
		}
	}
	
	// Add Kubernetes framework to server config
	if !contains(config.Frameworks, "kubernetes") {
		config.Frameworks = append(config.Frameworks, "kubernetes")
	}
	
	return nil
}

func (k *KubernetesEnhancer) GetRequiredSettings() map[string]interface{} {
	return map[string]interface{}{
		"buildFlags": []string{"-tags=ignore_autogenerated"},
	}
}

func (k *KubernetesEnhancer) GetRecommendedExtensions() []string {
	return []string{"yaml", "dockerfile", "helm"}
}

// Monorepo Strategies

type LanguageSeparatedStrategy struct{}

func (l *LanguageSeparatedStrategy) GenerateConfig(projectInfo *MultiLanguageProjectInfo) (*MultiLanguageConfig, error) {
	generator := NewConfigGenerator()
	return generator.GenerateMultiLanguageConfig(projectInfo)
}

func (l *LanguageSeparatedStrategy) GetWorkspaceLayout(layout *MonorepoLayout) map[string]string {
	workspaceLayout := make(map[string]string)
	
	for _, workspace := range layout.Workspaces {
		// Assume workspace names indicate language (e.g., "go-services", "python-libs")
		if strings.Contains(workspace, "go") {
			workspaceLayout["go"] = workspace
		} else if strings.Contains(workspace, "python") {
			workspaceLayout["python"] = workspace
		} else if strings.Contains(workspace, "typescript") || strings.Contains(workspace, "js") {
			workspaceLayout["typescript"] = workspace
		} else if strings.Contains(workspace, "java") {
			workspaceLayout["java"] = workspace
		}
	}
	
	return workspaceLayout
}

func (l *LanguageSeparatedStrategy) OptimizeForLayout(config *MultiLanguageConfig, layout *MonorepoLayout) error {
	// Each language gets its own workspace root
	workspaceLayout := l.GetWorkspaceLayout(layout)
	
	for _, serverConfig := range config.ServerConfigs {
		if len(serverConfig.Languages) > 0 {
			language := serverConfig.Languages[0]
			if workspaceRoot, exists := workspaceLayout[language]; exists {
				if serverConfig.WorkspaceRoots == nil {
					serverConfig.WorkspaceRoots = make(map[string]string)
				}
				serverConfig.WorkspaceRoots[language] = workspaceRoot
			}
		}
		
		// Optimize for language isolation
		serverConfig.ServerType = ServerTypeSingle
		serverConfig.Priority += 2 // Higher priority for isolated languages
	}
	
	config.WorkspaceConfig.CrossLanguageReferences = false
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyIncremental
	
	return nil
}

type MixedStrategy struct{}

func (m *MixedStrategy) GenerateConfig(projectInfo *MultiLanguageProjectInfo) (*MultiLanguageConfig, error) {
	generator := NewConfigGenerator()
	return generator.GenerateMultiLanguageConfig(projectInfo)
}

func (m *MixedStrategy) GetWorkspaceLayout(layout *MonorepoLayout) map[string]string {
	// In mixed strategy, all languages share workspace roots
	workspaceLayout := make(map[string]string)
	
	if len(layout.Workspaces) > 0 {
		sharedRoot := layout.Workspaces[0]
		workspaceLayout["shared"] = sharedRoot
	}
	
	return workspaceLayout
}

func (m *MixedStrategy) OptimizeForLayout(config *MultiLanguageConfig, layout *MonorepoLayout) error {
	sharedRoot := ""
	if len(layout.Workspaces) > 0 {
		sharedRoot = layout.Workspaces[0]
	}
	
	for _, serverConfig := range config.ServerConfigs {
		// All servers share the same workspace root
		if serverConfig.WorkspaceRoots == nil {
			serverConfig.WorkspaceRoots = make(map[string]string)
		}
		
		for _, language := range serverConfig.Languages {
			serverConfig.WorkspaceRoots[language] = sharedRoot
		}
		
		// Optimize for cross-language collaboration
		serverConfig.ServerType = ServerTypeMulti
		serverConfig.Priority += 1
	}
	
	config.WorkspaceConfig.CrossLanguageReferences = true
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategySmart
	config.WorkspaceConfig.SharedSettings["cross_language_navigation"] = true
	
	return nil
}

type MicroservicesStrategy struct{}

func (m *MicroservicesStrategy) GenerateConfig(projectInfo *MultiLanguageProjectInfo) (*MultiLanguageConfig, error) {
	generator := NewConfigGenerator()
	return generator.GenerateMultiLanguageConfig(projectInfo)
}

func (m *MicroservicesStrategy) GetWorkspaceLayout(layout *MonorepoLayout) map[string]string {
	workspaceLayout := make(map[string]string)
	
	// Each service gets its own workspace
	for i, workspace := range layout.Workspaces {
		serviceName := fmt.Sprintf("service-%d", i)
		workspaceLayout[serviceName] = workspace
	}
	
	return workspaceLayout
}

func (m *MicroservicesStrategy) OptimizeForLayout(config *MultiLanguageConfig, layout *MonorepoLayout) error {
	// Optimize for service isolation
	for _, serverConfig := range config.ServerConfigs {
		serverConfig.ServerType = ServerTypeSingle
		serverConfig.MaxConcurrentRequests = 25 // Lower for microservices
		
		// Each service is isolated
		if serverConfig.WorkspaceRoots == nil {
			serverConfig.WorkspaceRoots = make(map[string]string)
		}
		
		// Assign workspaces based on service patterns
		for i, workspace := range layout.Workspaces {
			serviceName := fmt.Sprintf("service-%d", i)
			for _, language := range serverConfig.Languages {
				serverConfig.WorkspaceRoots[serviceName] = workspace
			}
		}
	}
	
	config.WorkspaceConfig.CrossLanguageReferences = false
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyIncremental
	config.WorkspaceConfig.SharedSettings["service_isolation"] = true
	
	return nil
}

// Optimization Modes

type DevelopmentOptimization struct{}

func (d *DevelopmentOptimization) ApplyOptimizations(config *MultiLanguageConfig) error {
	config.OptimizedFor = OptimizationDevelopment
	
	// Enable development-friendly features
	for _, serverConfig := range config.ServerConfigs {
		// Enable diagnostics and detailed analysis
		language := serverConfig.Languages[0]
		switch language {
		case "go":
			if gopls, ok := serverConfig.Settings["gopls"].(map[string]interface{}); ok {
				gopls["verboseOutput"] = true
				if analyses, ok := gopls["analyses"].(map[string]bool); ok {
					analyses["unusedparams"] = true
					analyses["shadow"] = true
				}
			}
		case "python":
			if pylsp, ok := serverConfig.Settings["pylsp"].(map[string]interface{}); ok {
				if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
					plugins["pycodestyle"] = map[string]bool{"enabled": true}
					plugins["pyflakes"] = map[string]bool{"enabled": true}
				}
			}
		case "typescript":
			if ts, ok := serverConfig.Settings["typescript"].(map[string]interface{}); ok {
				if preferences, ok := ts["preferences"].(map[string]interface{}); ok {
					preferences["includeCompletionsForModuleExports"] = true
				}
			}
		}
		
		// Higher priority for development mode
		serverConfig.Priority += 1
	}
	
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyFull
	config.WorkspaceConfig.SharedSettings["development_mode"] = true
	
	return nil
}

func (d *DevelopmentOptimization) GetPerformanceSettings() map[string]interface{} {
	return map[string]interface{}{
		"indexing_strategy": "full",
		"enable_diagnostics": true,
		"verbose_output": true,
	}
}

func (d *DevelopmentOptimization) GetMemorySettings() map[string]interface{} {
	return map[string]interface{}{
		"memory_mode": "normal",
		"cache_enabled": true,
	}
}

type ProductionOptimization struct{}

func (p *ProductionOptimization) ApplyOptimizations(config *MultiLanguageConfig) error {
	config.OptimizedFor = OptimizationProduction
	
	// Optimize for performance and reduced resource usage
	for _, serverConfig := range config.ServerConfigs {
		language := serverConfig.Languages[0]
		switch language {
		case "go":
			if gopls, ok := serverConfig.Settings["gopls"].(map[string]interface{}); ok {
				gopls["memoryMode"] = "DegradeClosed"
				gopls["verboseOutput"] = false
				if analyses, ok := gopls["analyses"].(map[string]bool); ok {
					// Disable expensive analyses in production
					analyses["fieldalignment"] = false
					analyses["unusedwrite"] = false
				}
			}
		case "python":
			if pylsp, ok := serverConfig.Settings["pylsp"].(map[string]interface{}); ok {
				if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
					// Disable heavy plugins in production
					if rope, ok := plugins["rope_completion"].(map[string]interface{}); ok {
						rope["eager"] = false
					}
				}
			}
		case "typescript":
			if ts, ok := serverConfig.Settings["typescript"].(map[string]interface{}); ok {
				ts["disableAutomaticTypeAcquisition"] = true
				if preferences, ok := ts["preferences"].(map[string]interface{}); ok {
					preferences["includeCompletionsForModuleExports"] = false
				}
			}
		}
		
		// Lower concurrent requests for production stability
		if serverConfig.MaxConcurrentRequests == 0 {
			serverConfig.MaxConcurrentRequests = 50
		}
	}
	
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyIncremental
	config.WorkspaceConfig.SharedSettings["production_mode"] = true
	
	return nil
}

func (p *ProductionOptimization) GetPerformanceSettings() map[string]interface{} {
	return map[string]interface{}{
		"indexing_strategy": "incremental",
		"memory_mode": "degraded_closed",
		"disable_expensive_analyses": true,
	}
}

func (p *ProductionOptimization) GetMemorySettings() map[string]interface{} {
	return map[string]interface{}{
		"memory_mode": "conservative",
		"cache_enabled": false,
		"gc_aggressive": true,
	}
}

type AnalysisOptimization struct{}

func (a *AnalysisOptimization) ApplyOptimizations(config *MultiLanguageConfig) error {
	config.OptimizedFor = OptimizationAnalysis
	
	// Enable maximum analysis capabilities
	for _, serverConfig := range config.ServerConfigs {
		language := serverConfig.Languages[0]
		switch language {
		case "go":
			if gopls, ok := serverConfig.Settings["gopls"].(map[string]interface{}); ok {
				gopls["staticcheck"] = true
				gopls["verboseOutput"] = true
				if analyses, ok := gopls["analyses"].(map[string]bool); ok {
					// Enable all analyses for deep analysis
					for analysis := range analyses {
						analyses[analysis] = true
					}
				}
			}
		case "python":
			if pylsp, ok := serverConfig.Settings["pylsp"].(map[string]interface{}); ok {
				if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
					plugins["pylint"] = map[string]bool{"enabled": true}
					plugins["mypy-ls"] = map[string]interface{}{
						"enabled": true,
						"strict": true,
					}
				}
			}
		case "typescript":
			if ts, ok := serverConfig.Settings["typescript"].(map[string]interface{}); ok {
				if preferences, ok := ts["preferences"].(map[string]interface{}); ok {
					preferences["strictNullChecks"] = true
					preferences["strictFunctionTypes"] = true
					preferences["noImplicitReturns"] = true
					preferences["noImplicitAny"] = true
				}
			}
		}
		
		// Higher priority for analysis
		serverConfig.Priority += 3
		serverConfig.Weight += 1.0
	}
	
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyFull
	config.WorkspaceConfig.CrossLanguageReferences = true
	config.WorkspaceConfig.SharedSettings["analysis_mode"] = true
	config.WorkspaceConfig.SharedSettings["enable_all_diagnostics"] = true
	
	return nil
}

func (a *AnalysisOptimization) GetPerformanceSettings() map[string]interface{} {
	return map[string]interface{}{
		"indexing_strategy": "full",
		"enable_all_analyses": true,
		"deep_analysis": true,
	}
}

func (a *AnalysisOptimization) GetMemorySettings() map[string]interface{} {
	return map[string]interface{}{
		"memory_mode": "unlimited",
		"cache_enabled": true,
		"cache_size": "large",
	}
}

// Utility functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}