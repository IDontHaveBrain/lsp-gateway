#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

/**
 * Prepare binaries for npm publishing
 * Ensures Windows binary has the correct name for npm package
 */

const binDir = path.join(__dirname, '..', 'bin');

// Copy lsp-gateway-windows.exe to lsp-gateway.exe for npm compatibility
const windowsSource = path.join(binDir, 'lsp-gateway-windows.exe');
const windowsTarget = path.join(binDir, 'lsp-gateway.exe');

if (fs.existsSync(windowsSource)) {
  console.log('📦 Preparing Windows binary for npm...');
  fs.copyFileSync(windowsSource, windowsTarget);
  console.log(`✅ Copied ${path.basename(windowsSource)} to ${path.basename(windowsTarget)}`);
} else {
  console.log('⚠️  Windows binary not found, skipping...');
}

// Also ensure other platform binaries are in place
const platformMap = {
  'lsp-gateway-linux': 'lsp-gateway-linux',
  'lsp-gateway-macos': 'lsp-gateway-macos',
  'lsp-gateway-macos-arm64': 'lsp-gateway-macos-arm64'
};

for (const [source, target] of Object.entries(platformMap)) {
  const sourcePath = path.join(binDir, source);
  const targetPath = path.join(binDir, target);
  
  if (fs.existsSync(sourcePath) && sourcePath !== targetPath) {
    console.log(`✅ Binary ${source} is ready`);
  }
}

console.log('✅ Binaries prepared for npm publishing');