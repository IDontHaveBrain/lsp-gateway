package server

import (
    "runtime"
    "strings"

    "lsp-gateway/src/internal/common"
)

const osWindows = "windows"

func (m *LSPManager) resolveCommandPath2(language, command string) string { return m.resolveCommandPath(language, command) }

func (m *LSPManager) resolveCommandPath(language, command string) string {
    if root := common.GetLSPToolRoot(language); root != "" {
        if resolved := common.FirstExistingExecutable(root, []string{command}); resolved != "" {
            common.LSPLogger.Debug("Using installed %s command at %s", language, resolved)
            return resolved
        }
    }
    if language == langJava {
        if p := m.resolveJavaCustom(command); p != "" { return p }
        if p := m.resolveWindowsJDTLS(command); p != "" { return p }
    }
    if p := m.resolveCustomLanguageMap(command); p != "" { return p }
    return command
}

func (m *LSPManager) resolveJavaCustom(command string) string {
    if command != "jdtls" { return "" }
    var customPath string
    if runtime.GOOS == osWindows { customPath = common.GetLSPToolPath("java", "jdtls.bat") } else { customPath = common.GetLSPToolPath("java", "jdtls") }
    if common.FileExists(customPath) { common.LSPLogger.Debug("Using custom jdtls installation at %s", customPath); return customPath }
    return ""
}

func (m *LSPManager) resolveWindowsJDTLS(command string) string {
    if runtime.GOOS != osWindows { return "" }
    lower := strings.ToLower(command)
    if strings.HasSuffix(lower, "\\jdtls") || strings.HasSuffix(lower, "/jdtls") || lower == "jdtls" {
        if common.FileExists(command+".bat") { return command+".bat" }
        if common.FileExists(command+".cmd") { return command+".cmd" }
    }
    return ""
}

func (m *LSPManager) resolveCustomLanguageMap(command string) string {
    switch command {
    case "gopls", "pylsp", serverJediLS, serverPyrightLS, serverBasedPyrightLS,
        "typescript-language-server", "omnisharp", "OmniSharp", "kotlin-lsp", "kotlin-language-server":
    default:
        return ""
    }
    languageMap := map[string]string{
        "gopls": "go",
        "pylsp": "python",
        serverJediLS: "python",
        serverPyrightLS: "python",
        serverBasedPyrightLS: "python",
        "typescript-language-server": "typescript",
        "omnisharp": "csharp",
        "OmniSharp": "csharp",
        "kotlin-lsp": "kotlin",
        "kotlin-language-server": "kotlin",
    }
    lang, exists := languageMap[command]
    if !exists { return "" }
    customPath := common.GetLSPToolPath(lang, command)
    if runtime.GOOS == osWindows {
        switch command {
        case "typescript-language-server", "pylsp", "jedi-language-server":
            customPath = customPath + ".cmd"
        case "omnisharp", "OmniSharp":
            customPath = customPath + ".exe"
        case "kotlin-lsp":
            exePath := common.GetLSPToolPath("kotlin", "kotlin-lsp.exe")
            if common.FileExists(exePath) { customPath = exePath } else { customPath = customPath + ".cmd" }
        case "kotlin-language-server":
            batPath := common.GetLSPToolPath("kotlin", "kotlin-language-server.bat")
            if common.FileExists(batPath) { customPath = batPath } else {
                exePath := common.GetLSPToolPath("kotlin", "kotlin-language-server.exe")
                if common.FileExists(exePath) { customPath = exePath }
            }
        }
    }
    if common.FileExists(customPath) { common.LSPLogger.Debug("Using custom %s installation at %s", command, customPath); return customPath }
    return ""
}

