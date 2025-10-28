#!/bin/bash
# Cleanup orphaned LSP server processes that may leak during test runs
# This script kills LSP servers and related processes that are no longer needed

set +e  # Continue on errors

echo "🧹 Cleaning up orphaned LSP server processes..."

# Kill Kotlin LSP servers (JVM-based, very memory intensive)
if pgrep -f "KotlinLspServerKt" > /dev/null; then
    pkill -9 -f "KotlinLspServerKt"
    echo "✓ Killed Kotlin LSP servers"
else
    echo "  No Kotlin LSP servers found"
fi

# Kill Java JDTLS servers
if pgrep -f "jdtls" > /dev/null; then
    pkill -9 -f "jdtls"
    echo "✓ Killed JDTLS servers"
else
    echo "  No JDTLS servers found"
fi

# Kill gopls telemetry processes (often left behind)
if pgrep -f "gopls.*telemetry" > /dev/null; then
    pkill -9 -f "gopls.*telemetry"
    echo "✓ Killed gopls telemetry processes"
fi

# Kill gopls main processes
if pgrep gopls > /dev/null; then
    pkill -9 gopls
    echo "✓ Killed gopls servers"
else
    echo "  No gopls servers found"
fi

# Kill OmniSharp servers (C#)
if pgrep omnisharp > /dev/null; then
    pkill -9 omnisharp
    echo "✓ Killed OmniSharp servers"
else
    echo "  No OmniSharp servers found"
fi

# Kill rust-analyzer and its proc-macro-srv
if pgrep rust-analyzer > /dev/null; then
    pkill -9 rust-analyzer
    pkill -9 -f "rust-analyzer-proc-macro-srv"
    echo "✓ Killed rust-analyzer servers"
else
    echo "  No rust-analyzer servers found"
fi

# Kill Python LSP servers
if pgrep -f "basedpyright\|pyright\|jedi-language-server\|pylsp" > /dev/null; then
    pkill -9 -f "basedpyright"
    pkill -9 -f "pyright"
    pkill -9 -f "jedi-language-server"
    pkill -9 -f "pylsp"
    echo "✓ Killed Python LSP servers"
else
    echo "  No Python LSP servers found"
fi

# Kill TypeScript language servers
if pgrep -f "typescript-language-server" > /dev/null; then
    pkill -9 -f "typescript-language-server"
    echo "✓ Killed TypeScript language servers"
else
    echo "  No TypeScript language servers found"
fi

# Kill Gradle daemons (often left behind by Java tests)
if pgrep -f "gradle.*daemon" > /dev/null; then
    pkill -9 -f "gradle.*daemon"
    echo "✓ Killed Gradle daemons"
else
    echo "  No Gradle daemons found"
fi

# Kill any lsp-gateway server processes (from tests)
if pgrep -f "lsp-gateway.*server" > /dev/null; then
    pkill -9 -f "lsp-gateway.*server"
    echo "✓ Killed lsp-gateway server processes"
else
    echo "  No lsp-gateway servers found"
fi

echo ""
echo "✅ Cleanup complete!"
echo ""

# Show memory usage after cleanup
echo "💾 Current memory usage:"
free -h | grep "Mem:" | awk '{printf "   Used: %s / %s (Available: %s)\n", $3, $2, $7}'
