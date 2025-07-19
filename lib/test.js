#!/usr/bin/env node

/**
 * Test script for LSP Gateway NPM package
 * Tests platform detection and package structure
 */

const PlatformInfo = require('./platform');
const { LSPGateway } = require('./index');

console.log('🧪 Testing LSP Gateway NPM Package...\n');

// Test platform detection
console.log('📦 Platform Detection Test:');
try {
  const platform = new PlatformInfo();
  console.log(`  ✅ Current platform: ${platform.getCurrentPlatform()}`);
  console.log(`  ✅ Platform name: ${platform.getPlatformName()}`);
  console.log(`  ✅ Supported: ${platform.isSupported()}`);
  console.log(`  ✅ Binary path: ${platform.getBinaryPath()}`);
  console.log(`  ✅ Windows: ${platform.isWindows()}`);
  console.log(`  ✅ macOS: ${platform.isMacOS()}`);
  console.log(`  ✅ Linux: ${platform.isLinux()}`);
  console.log(`  ✅ Apple Silicon: ${platform.isAppleSilicon()}`);
  console.log(`  ✅ Supported platforms: ${platform.getSupportedPlatforms().join(', ')}`);
} catch (error) {
  console.error(`  ❌ Platform detection failed: ${error.message}`);
}

console.log('\n🚀 LSP Gateway API Test:');
try {
  const gateway = new LSPGateway({
    port: 8080,
    config: './config.yaml'
  });
  
  console.log('  ✅ Gateway instance created');
  console.log(`  ✅ Binary path: ${gateway.getBinaryPath()}`);
  
  const status = gateway.getStatus();
  console.log(`  ✅ Status: ${JSON.stringify(status, null, 2)}`);
  
  const platformInfo = gateway.getPlatformInfo();
  console.log(`  ✅ Platform info: ${JSON.stringify(platformInfo, null, 2)}`);
  
} catch (error) {
  console.error(`  ❌ Gateway API test failed: ${error.message}`);
}

console.log('\n📋 Package Structure Test:');
const fs = require('fs');
const path = require('path');

const expectedFiles = [
  '../package.json',
  '../config.yaml',
  '../bin/lsp-gateway',
  './platform.js',
  './installer.js',
  './index.js'
];

expectedFiles.forEach(file => {
  const filePath = path.join(__dirname, file);
  if (fs.existsSync(filePath)) {
    console.log(`  ✅ ${file} exists`);
  } else {
    console.log(`  ❌ ${file} missing`);
  }
});

console.log('\n🔍 Binary Availability Test:');
try {
  const platform = new PlatformInfo();
  const binaryPath = platform.getBinaryPath();
  
  if (fs.existsSync(binaryPath)) {
    console.log('  ✅ Binary exists');
    
    // Test if binary is executable
    const { spawn } = require('child_process');
    const child = spawn(binaryPath, ['--version'], {
      stdio: 'pipe',
      timeout: 5000
    });
    
    let output = '';
    child.stdout.on('data', (data) => {
      output += data.toString();
    });
    
    child.on('close', (code) => {
      if (code === 0) {
        console.log('  ✅ Binary is executable');
        console.log(`  ✅ Version output: ${output.trim()}`);
      } else {
        console.log(`  ⚠️  Binary exited with code ${code}`);
      }
    });
    
    child.on('error', (error) => {
      console.log(`  ❌ Binary execution failed: ${error.message}`);
    });
    
  } else {
    console.log('  ⚠️  Binary not found (will be installed on first run)');
  }
} catch (error) {
  console.error(`  ❌ Binary test failed: ${error.message}`);
}

console.log('\n✅ NPM Package tests completed!');
console.log('\n📚 Usage examples:');
console.log('  # Install globally');
console.log('  npm install -g lsp-gateway');
console.log('');
console.log('  # Run CLI');
console.log('  lsp-gateway server --port 8080');
console.log('');
console.log('  # Use programmatically');
console.log('  const { LSPGateway } = require("lsp-gateway");');
console.log('  const gateway = new LSPGateway({ port: 8080 });');
console.log('  await gateway.start();');