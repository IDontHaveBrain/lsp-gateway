package common

import (
	"os"
	"runtime"
)

const trueStr = "true"

func IsCI() bool {
	return os.Getenv("CI") == trueStr || os.Getenv("GITHUB_ACTIONS") == trueStr
}

func IsWindowsCI() bool {
	return runtime.GOOS == "windows" && IsCI()
}
