package cache

import (
	"runtime"
	"strings"
)

// computeWorkers returns a bounded worker count with Java/Windows handling
func computeWorkers(hasJava bool) int {
	workers := runtime.NumCPU()
	if hasJava && runtime.GOOS == "windows" {
		return 1
	}
	if hasJava {
		if workers > 2 {
			return 2
		}
		if workers < 1 {
			return 1
		}
		return workers
	}
	if workers < 2 {
		workers = 2
	}
	if workers > 16 {
		workers = 16
	}
	return workers
}

func hasJavaInLangs(languages []string) bool {
	for _, lang := range languages {
		if lang == "java" {
			return true
		}
	}
	return false
}

func hasJavaInFiles(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".java") {
			return true
		}
	}
	return false
}

func hasJavaInURIs(uris []string) bool {
	for _, u := range uris {
		if strings.HasSuffix(strings.ToLower(u), ".java") {
			return true
		}
	}
	return false
}
