#!/usr/bin/env node

/**
 * Build script wrapper for LSP Gateway
 * 
 * This Node.js wrapper calls the underlying Go build system (Makefile)
 * to maintain zero Node.js dependencies while providing npm ecosystem integration.
 * 
 * Usage:
 *   npm run build           - Build for all platforms
 *   npm run build local     - Build for current platform only
 *   npm run build clean     - Clean and rebuild
 */

const { spawn } = require('child_process');
const process = require('process');
const path = require('path');

// Colors for output
const colors = {
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  reset: '\x1b[0m'
};

function log(color, message) {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function checkMakeAvailable() {
  return new Promise((resolve) => {
    const child = spawn('make', ['--version'], { stdio: 'ignore' });
    child.on('close', (code) => {
      resolve(code === 0);
    });
    child.on('error', () => {
      resolve(false);
    });
  });
}

function runCommand(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    log('blue', `Running: ${command} ${args.join(' ')}`);
    
    const child = spawn(command, args, {
      stdio: 'inherit',
      cwd: path.join(__dirname, '..'),
      ...options
    });

    child.on('close', (code) => {
      if (code === 0) {
        resolve(code);
      } else {
        reject(new Error(`Command failed with exit code ${code}`));
      }
    });

    child.on('error', (error) => {
      reject(error);
    });
  });
}

async function main() {
  const args = process.argv.slice(2);
  const target = args[0] || 'build';

  log('yellow', 'LSP Gateway Build Script');
  log('yellow', '======================');

  try {
    // Check if make is available
    const makeAvailable = await checkMakeAvailable();
    
    if (!makeAvailable) {
      log('red', 'Error: make command not found');
      log('yellow', 'Please install make or use the build.sh script directly:');
      log('yellow', '  bash scripts/build.sh');
      process.exit(1);
    }

    // Handle different build targets
    switch (target) {
      case 'clean':
        log('blue', 'Cleaning and rebuilding...');
        await runCommand('make', ['clean']);
        await runCommand('make', ['build']);
        break;
      
      case 'local':
        log('blue', 'Building for current platform...');
        await runCommand('make', ['local']);
        break;
      
      case 'linux':
      case 'windows':
      case 'macos':
      case 'macos-arm64':
        log('blue', `Building for ${target}...`);
        await runCommand('make', [target]);
        break;
      
      case 'build':
      default:
        log('blue', 'Building for all platforms...');
        await runCommand('make', ['build']);
        break;
    }

    log('green', '✓ Build completed successfully!');
    
    // Show build info
    log('yellow', '\nBuild artifacts:');
    await runCommand('make', ['check']);

  } catch (error) {
    log('red', `✗ Build failed: ${error.message}`);
    
    // Fallback to bash script if make fails
    log('yellow', '\nAttempting fallback to bash script...');
    try {
      await runCommand('bash', ['scripts/build.sh']);
      log('green', '✓ Fallback build completed successfully!');
    } catch (fallbackError) {
      log('red', `✗ Fallback also failed: ${fallbackError.message}`);
      log('yellow', '\nPlease ensure you have Go installed and try:');
      log('yellow', '  go build -o bin/lsp-gateway cmd/lsp-gateway/main.go');
      process.exit(1);
    }
  }
}

// Handle SIGINT gracefully
process.on('SIGINT', () => {
  log('yellow', '\nBuild interrupted by user');
  process.exit(130);
});

// Run main function
main().catch((error) => {
  log('red', `Unexpected error: ${error.message}`);
  process.exit(1);
});