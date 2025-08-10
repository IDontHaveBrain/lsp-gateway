package utils

import (
	"golang.org/x/sys/windows"
)

func LongPath(p string) string {
	if p == "" {
		return p
	}
	uptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetLongPathName(uptr, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return p
	}
	if int(n) > len(buf) {
		return p
	}
	return windows.UTF16ToString(buf[:n])
}
