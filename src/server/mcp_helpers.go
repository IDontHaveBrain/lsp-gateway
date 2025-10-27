package server

import (
	"bufio"
	"os"
	"strings"
)

// extractCodeLines reads the source file and returns the lines between startLine and endLine (inclusive).
// Lines are zero-indexed in internal representations, so we treat the provided values as zero-based.
func extractCodeLines(filePath string, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var lines []string

	current := 0
	for scanner.Scan() {
		if current >= startLine && current <= endLine {
			lines = append(lines, scanner.Text())
		}
		if current > endLine {
			break
		}
		current++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
