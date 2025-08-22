package common

import (
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
)

func PlatformExpand(names []string) []string {
    out := make([]string, 0, len(names)*4)
    for _, n := range names {
        if runtime.GOOS == "windows" {
            ext := filepath.Ext(n)
            if ext == ".cmd" || ext == ".bat" || ext == ".exe" {
                out = append(out, n)
            } else {
                out = append(out, n, n+".cmd", n+".bat", n+".exe")
            }
        } else {
            if filepath.Ext(n) == "" {
                out = append(out, n, n+".sh")
            } else {
                out = append(out, n)
            }
        }
    }
    seen := map[string]struct{}{}
    uniq := make([]string, 0, len(out))
    for _, v := range out {
        if _, ok := seen[v]; ok {
            continue
        }
        seen[v] = struct{}{}
        uniq = append(uniq, v)
    }
    return uniq
}

func FirstExistingExecutable(installRoot string, names []string) string {
    cands := PlatformExpand(names)
    for _, n := range cands {
        if _, err := exec.LookPath(n); err == nil {
            return n
        }
    }
    for _, n := range cands {
        p := filepath.Join(installRoot, n)
        if info, err := os.Stat(p); err == nil {
            if runtime.GOOS == "windows" {
                return p
            }
            if info.Mode()&0111 != 0 {
                return p
            }
        }
    }
    for _, n := range cands {
        p := filepath.Join(installRoot, "bin", n)
        if info, err := os.Stat(p); err == nil {
            if runtime.GOOS == "windows" {
                return p
            }
            if info.Mode()&0111 != 0 {
                return p
            }
        }
    }
    return ""
}

func HasAnyExecutable(installRoot string, names []string) bool {
    return FirstExistingExecutable(installRoot, names) != ""
}

