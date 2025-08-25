package main

import (
	"fmt"
	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/project"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: checkdetect <dir>")
		os.Exit(1)
	}
	dir := os.Args[1]
	langs, err := project.DetectLanguages(dir)
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Println("detected:", langs)
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"kotlin": {Command: "kotlin-lsp", Args: []string{}},
		},
	}
	set := map[string]bool{}
	for _, l := range langs {
		set[l] = true
	}
	var toStart []string
	for lang := range cfg.Servers {
		if set[lang] {
			toStart = append(toStart, lang)
		}
	}
	fmt.Println("toStart:", toStart)
}
