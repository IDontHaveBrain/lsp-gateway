package main

import (
	"fmt"
	"os"
	"lsp-gateway/internal/installer"
)

func main() {
	path := installer.GetJDTLSExecutablePath()
	fmt.Printf("JDTLS executable path: %s\n", path)
	
	if _, err := os.Stat(path); err == nil {
		fmt.Println("✅ JDTLS is available and detected!")
	} else {
		fmt.Printf("❌ JDTLS not found: %v\n", err)
	}
}