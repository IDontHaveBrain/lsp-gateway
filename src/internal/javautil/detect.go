package javautil

import (
    "os"
    "path/filepath"
    "runtime"
    "strings"

    "lsp-gateway/src/config"
    "lsp-gateway/src/internal/common"
)

func ExeName(name string) string {
    if runtime.GOOS == "windows" {
        return name + ".exe"
    }
    return name
}

func FindLocalJavaExecutable(base string) string {
    candidates := []string{
        filepath.Join(base, "tools", "java", "jdk", "current", "bin", ExeName("java")),
        filepath.Join(base, "jdk", "current", "bin", ExeName("java")),
    }

    for _, c := range candidates {
        if common.FileExists(c) {
            return c
        }
    }

    entries, err := os.ReadDir(base)
    if err == nil {
        for _, e := range entries {
            if !e.IsDir() {
                continue
            }
            name := e.Name()
            low := strings.ToLower(name)
            if strings.Contains(low, "jdk") || strings.HasPrefix(low, "java") {
                p := filepath.Join(base, name, "bin", ExeName("java"))
                if common.FileExists(p) {
                    return p
                }
            }
        }
    }

    return ""
}

func FindLocalJDTLSLauncher(base string) (string, string) {
    jdtlsBase := filepath.Join(base, "tools", "java", "jdtls")
    if !common.FileExists(jdtlsBase) {
        alt := filepath.Join(base, "jdtls")
        if common.FileExists(alt) {
            jdtlsBase = alt
        }
    }

    pluginsDir := filepath.Join(jdtlsBase, "plugins")
    if fi, err := os.Stat(pluginsDir); err != nil || !fi.IsDir() {
        return "", ""
    }

    var launcher string
    if entries, err := os.ReadDir(pluginsDir); err == nil {
        for _, e := range entries {
            if e.IsDir() {
                continue
            }
            name := e.Name()
            if strings.HasPrefix(name, "org.eclipse.equinox.launcher_") && strings.HasSuffix(name, ".jar") {
                launcher = filepath.Join(pluginsDir, name)
                break
            }
        }
    }
    if launcher == "" {
        return "", ""
    }

    var cfg string
    switch runtime.GOOS {
    case "windows":
        cfg = filepath.Join(jdtlsBase, "config_win")
    case "darwin":
        cfg = filepath.Join(jdtlsBase, "config_mac")
    default:
        cfg = filepath.Join(jdtlsBase, "config_linux")
    }

    if !common.FileExists(cfg) {
        return "", ""
    }
    return launcher, cfg
}

func BuildJDTLSArgs(launcherJar, configDir, workspace string) []string {
    return []string{
        "-Declipse.application=org.eclipse.jdt.ls.core.id1",
        "-Dosgi.bundles.defaultStartLevel=4",
        "-Declipse.product=org.eclipse.jdt.ls.core.product",
        "-Dlog.level=ALL",
        "-noverify",
        "-Xmx1G",
        "-jar", launcherJar,
        "-configuration", configDir,
        "-data", workspace,
        "--add-modules=ALL-SYSTEM",
        "--add-opens", "java.base/java.util=ALL-UNNAMED",
        "--add-opens", "java.base/java.lang=ALL-UNNAMED",
    }
}

func ConfigureJavaFromLocalDownloads(homeDir string, javaServer *config.ServerConfig) bool {
    base := filepath.Join(homeDir, ".lsp-gateway")
    javaExe := FindLocalJavaExecutable(base)
    if javaExe == "" {
        return false
    }
    launcherJar, configDir := FindLocalJDTLSLauncher(base)
    if launcherJar == "" || configDir == "" {
        return false
    }
    workspace := filepath.Join(base, "workspace")
    javaServer.Command = javaExe
    javaServer.Args = BuildJDTLSArgs(launcherJar, configDir, workspace)
    return true
}

