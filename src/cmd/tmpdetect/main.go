package main
import (
	"fmt"
	"os"
	"lsp-gateway/src/internal/project"
)
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: tmpdetect <dir>")
		os.Exit(1)
	}
	langs, err := project.DetectLanguages(os.Args[1])
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Println(langs)
}
