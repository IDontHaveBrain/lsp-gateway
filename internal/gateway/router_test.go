package gateway

import (
	"testing"
)

func TestNewRouter(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	if router == nil {
		t.Fatal("NewRouter() returned nil")
	}

	if router.langToServer == nil {
		t.Fatal("langToServer map is nil")
	}

	if router.extToLang == nil {
		t.Fatal("extToLang map is nil")
	}
}

func TestRegisterServer(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	router.RegisterServer("gopls", []string{"go"})

	server, exists := router.GetServerByLanguage("go")
	if !exists {
		t.Fatal("Go language not registered")
	}

	if server != "gopls" {
		t.Fatalf("Expected gopls, got %s", server)
	}

	lang, exists := router.GetLanguageByExtension("go")
	if !exists {
		t.Fatal("Go extension not registered")
	}

	if lang != "go" {
		t.Fatalf("Expected go, got %s", lang)
	}
}

func TestRouteRequest(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	router.RegisterServer("gopls", []string{"go"})
	router.RegisterServer("pyright", []string{"python"})
	router.RegisterServer("typescript-language-server", []string{"typescript", "javascript"})

	tests := []struct {
		name           string
		uri            string
		expectedServer string
		shouldError    bool
	}{
		{
			name:           "Go file",
			uri:            "file:///path/to/main.go",
			expectedServer: "gopls",
			shouldError:    false,
		},
		{
			name:           "Python file",
			uri:            "file:///path/to/script.py",
			expectedServer: "pyright",
			shouldError:    false,
		},
		{
			name:           "TypeScript file",
			uri:            "file:///path/to/component.ts",
			expectedServer: "typescript-language-server",
			shouldError:    false,
		},
		{
			name:           "JavaScript file",
			uri:            "file:///path/to/script.js",
			expectedServer: "typescript-language-server",
			shouldError:    false,
		},
		{
			name:           "Go mod file",
			uri:            "file:///path/to/go.mod",
			expectedServer: "gopls",
			shouldError:    false,
		},
		{
			name:           "Python type stub file",
			uri:            "file:///path/to/typing.pyi",
			expectedServer: "pyright",
			shouldError:    false,
		},
		{
			name:           "JSX file",
			uri:            "file:///path/to/component.jsx",
			expectedServer: "typescript-language-server",
			shouldError:    false,
		},
		{
			name:           "TSX file",
			uri:            "file:///path/to/component.tsx",
			expectedServer: "typescript-language-server",
			shouldError:    false,
		},
		{
			name:           "Plain URI without file:// prefix",
			uri:            "/path/to/main.go",
			expectedServer: "gopls",
			shouldError:    false,
		},
		{
			name:           "Unsupported extension",
			uri:            "file:///path/to/file.xyz",
			expectedServer: "",
			shouldError:    true,
		},
		{
			name:           "No extension",
			uri:            "file:///path/to/file",
			expectedServer: "",
			shouldError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := router.RouteRequest(tt.uri)

			if tt.shouldError {
				if err == nil {
					t.Fatalf("Expected error for %s, but got none", tt.uri)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error for %s: %v", tt.uri, err)
			}

			if server != tt.expectedServer {
				t.Fatalf("Expected %s, got %s for %s", tt.expectedServer, server, tt.uri)
			}
		})
	}
}

func TestGetExtensionsForLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		language string
		expected []string
	}{
		{
			language: "go",
			expected: []string{"go", "mod", "sum", "work"},
		},
		{
			language: "python",
			expected: []string{"py", "pyi", "pyx", "pyz", "pyw", "pyc", "pyo", "pyd"},
		},
		{
			language: "typescript",
			expected: []string{"ts", "tsx", "mts", "cts"},
		},
		{
			language: "javascript",
			expected: []string{"js", "jsx", "mjs", "cjs", "es", "es6", "es2015", "es2017", "es2018", "es2019", "es2020", "es2021", "es2022"},
		},
		{
			language: "java",
			expected: []string{"java", "class", "jar", "war", "ear", "jsp", "jspx"},
		},
		{
			language: "rust",
			expected: []string{"rs", "rlib"},
		},
		{
			language: "nonexistent",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			extensions := getExtensionsForLanguage(tt.language)

			if len(extensions) != len(tt.expected) {
				t.Fatalf("Expected %d extensions for %s, got %d", len(tt.expected), tt.language, len(extensions))
			}

			for i, ext := range extensions {
				if i < len(tt.expected) && ext != tt.expected[i] {
					t.Fatalf("Expected %s, got %s at index %d for %s", tt.expected[i], ext, i, tt.language)
				}
			}
		})
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	languages := router.GetSupportedLanguages()
	if len(languages) != 0 {
		t.Fatalf("Expected 0 languages, got %d", len(languages))
	}

	router.RegisterServer("gopls", []string{"go"})
	router.RegisterServer("pyright", []string{"python"})

	languages = router.GetSupportedLanguages()
	if len(languages) != 2 {
		t.Fatalf("Expected 2 languages, got %d", len(languages))
	}

	languageSet := make(map[string]bool)
	for _, lang := range languages {
		languageSet[lang] = true
	}

	if !languageSet["go"] || !languageSet["python"] {
		t.Fatal("Expected both go and python to be supported")
	}
}

func TestGetSupportedExtensions(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	extensions := router.GetSupportedExtensions()
	if len(extensions) != 0 {
		t.Fatalf("Expected 0 extensions, got %d", len(extensions))
	}

	router.RegisterServer("gopls", []string{"go"})

	extensions = router.GetSupportedExtensions()
	if len(extensions) == 0 {
		t.Fatal("Expected some extensions, got 0")
	}

	extensionSet := make(map[string]bool)
	for _, ext := range extensions {
		extensionSet[ext] = true
	}

	if !extensionSet["go"] {
		t.Fatal("Expected go extension to be supported")
	}
}

func TestGetLanguageByExtension(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	router.RegisterServer("gopls", []string{"go"})
	router.RegisterServer("pyright", []string{"python"})

	tests := []struct {
		extension    string
		expectedLang string
		shouldExist  bool
	}{
		{
			extension:    "go",
			expectedLang: "go",
			shouldExist:  true,
		},
		{
			extension:    ".go", // With dot
			expectedLang: "go",
			shouldExist:  true,
		},
		{
			extension:    "GO", // Case insensitive
			expectedLang: "go",
			shouldExist:  true,
		},
		{
			extension:    "py",
			expectedLang: "python",
			shouldExist:  true,
		},
		{
			extension:    "mod",
			expectedLang: "go",
			shouldExist:  true,
		},
		{
			extension:    "xyz",
			expectedLang: "",
			shouldExist:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			lang, exists := router.GetLanguageByExtension(tt.extension)

			if tt.shouldExist {
				if !exists {
					t.Fatalf("Expected language to exist for extension %s", tt.extension)
				}
				if lang != tt.expectedLang {
					t.Fatalf("Expected %s, got %s for extension %s", tt.expectedLang, lang, tt.extension)
				}
			} else {
				if exists {
					t.Fatalf("Expected no language for extension %s, but got %s", tt.extension, lang)
				}
			}
		})
	}
}

func TestGetServerByLanguage(t *testing.T) {
	t.Parallel()
	router := NewRouter()

	router.RegisterServer("gopls", []string{"go"})
	router.RegisterServer("pyright", []string{"python"})
	router.RegisterServer("typescript-language-server", []string{"typescript", "javascript"})

	tests := []struct {
		language       string
		expectedServer string
		shouldExist    bool
	}{
		{
			language:       "go",
			expectedServer: "gopls",
			shouldExist:    true,
		},
		{
			language:       "python",
			expectedServer: "pyright",
			shouldExist:    true,
		},
		{
			language:       "typescript",
			expectedServer: "typescript-language-server",
			shouldExist:    true,
		},
		{
			language:       "javascript",
			expectedServer: "typescript-language-server",
			shouldExist:    true,
		},
		{
			language:       "nonexistent",
			expectedServer: "",
			shouldExist:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			server, exists := router.GetServerByLanguage(tt.language)

			if tt.shouldExist {
				if !exists {
					t.Fatalf("Expected server to exist for language %s", tt.language)
				}
				if server != tt.expectedServer {
					t.Fatalf("Expected %s, got %s for language %s", tt.expectedServer, server, tt.language)
				}
			} else {
				if exists {
					t.Fatalf("Expected no server for language %s, but got %s", tt.language, server)
				}
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	router := NewRouter()

	router.RegisterServer("gopls", []string{"go"})
	router.RegisterServer("pyright", []string{"python"})

	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			router.GetSupportedLanguages()
			router.GetSupportedExtensions()
			router.GetServerByLanguage("go")
			router.GetLanguageByExtension("py")
			_, _ = router.RouteRequest("file:///test.go")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			router.RegisterServer("test-server", []string{"test"})
		}
		done <- true
	}()

	<-done
	<-done

	t.Log("Concurrent access test passed")
}
