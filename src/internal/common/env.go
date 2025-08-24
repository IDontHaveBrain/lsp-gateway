package common

import "os"

const trueStr = "true"

func IsCI() bool {
	return os.Getenv("CI") == trueStr || os.Getenv("GITHUB_ACTIONS") == trueStr
}
