const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');
const { spawn } = require('child_process');
const { promisify } = require('util');
const PlatformInfo = require('./platform');

const chmod = promisify(fs.chmod);
const copyFile = promisify(fs.copyFile);
const access = promisify(fs.access);
const unlink = promisify(fs.unlink);
const mkdir = promisify(fs.mkdir);
const stat = promisify(fs.stat);

/**
 * Binary installer for LSP Gateway
 * Downloads and installs the appropriate binary for the current platform
 */

class Installer {
  constructor() {
    this.platform = new PlatformInfo();
    this.packageJson = require('../package.json');
    this.version = this.packageJson.version;
    this.tempFiles = [];
    this.installStartTime = Date.now();
    this.skipChecksumVerification = process.env.SKIP_CHECKSUM_VERIFICATION === 'true';
    
    // SHA-256 checksums for binary integrity verification
    // TODO: These checksums need to be updated for each release
    this.checksums = {
      'v1.0.0': {
        'linux-x64': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
        'win32-x64': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
        'darwin-x64': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
        'darwin-arm64': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
      }
      // Additional versions can be added here as they are released
    };
  }

  /**
   * Calculate SHA-256 checksum of a file
   * @param {string} filePath - Path to the file
   * @returns {Promise<string>} SHA-256 hash in hexadecimal
   */
  async calculateFileChecksum(filePath) {
    return new Promise((resolve, reject) => {
      const hash = crypto.createHash('sha256');
      const stream = fs.createReadStream(filePath);
      
      stream.on('data', (data) => {
        hash.update(data);
      });
      
      stream.on('end', () => {
        resolve(hash.digest('hex'));
      });
      
      stream.on('error', (error) => {
        reject(new Error(`Failed to calculate checksum: ${error.message}`));
      });
    });
  }

  /**
   * Get expected checksum for current version and platform
   * @returns {string|null} Expected SHA-256 checksum or null if not found
   */
  getExpectedChecksum() {
    const versionString = this.version.startsWith('v') ? this.version : `v${this.version}`;
    const platformKey = this.platform.getCurrentPlatformString();
    
    if (this.checksums[versionString] && this.checksums[versionString][platformKey]) {
      return this.checksums[versionString][platformKey];
    }
    
    return null;
  }

  /**
   * Verify binary integrity using SHA-256 checksum
   * @param {string} binaryPath - Path to the binary file
   * @returns {Promise<boolean>} True if checksum matches, false otherwise
   */
  async verifyBinaryChecksum(binaryPath) {
    if (this.skipChecksumVerification) {
      console.log('⚠️  WARNING: Checksum verification is disabled via SKIP_CHECKSUM_VERIFICATION environment variable');
      console.log('   This is NOT recommended for production use!');
      return true;
    }

    const expectedChecksum = this.getExpectedChecksum();
    if (!expectedChecksum) {
      console.log(`⚠️  No checksum available for version ${this.version} on ${this.platform.getCurrentPlatformString()}`);
      console.log('   Proceeding without checksum verification');
      console.log('   WARNING: This reduces security against man-in-the-middle attacks');
      return true;
    }

    console.log('🔐 Verifying binary integrity...');
    
    try {
      const actualChecksum = await this.calculateFileChecksum(binaryPath);
      
      if (actualChecksum === expectedChecksum) {
        console.log('✅ Binary checksum verification successful');
        return true;
      } else {
        console.error('❌ Binary checksum verification FAILED!');
        console.error(`   Expected: ${expectedChecksum}`);
        console.error(`   Actual:   ${actualChecksum}`);
        console.error('   This indicates the binary may have been tampered with or corrupted');
        return false;
      }
    } catch (error) {
      console.error(`❌ Checksum verification failed: ${error.message}`);
      return false;
    }
  }

  /**
   * Main installation method
   * @returns {Promise<void>}
   */
  async install() {
    try {
      console.log('🔧 Installing LSP Gateway...');
      
      // Check platform support
      if (!this.platform.isSupported()) {
        throw new Error(`Platform ${this.platform.getCurrentPlatformString()} is not supported`);
      }

      console.log(`📦 Detected platform: ${this.platform.getPlatformName()}`);
      
      // Create bin directory if it doesn't exist
      const binDir = path.join(__dirname, '..', 'bin');
      await this.ensureDirectory(binDir);

      // Check if binary already exists and works
      const binaryPath = this.platform.getBinaryPath();
      if (await this.fileExists(binaryPath)) {
        console.log('✅ Binary already exists, checking if it works...');
        if (await this.testBinary(binaryPath)) {
          const duration = Date.now() - this.installStartTime;
          console.log(`✅ LSP Gateway is already installed and working! (${duration}ms)`);
          return;
        }
        console.log('⚠️  Existing binary is not working, reinstalling...');
        await this.cleanup([binaryPath]);
      }

      // Try multiple installation methods in order of preference
      const installMethods = [
        { method: () => this.installFromBuildOutput(binaryPath), name: 'build-output', requiresChecksum: false },
        { method: () => this.installFromLocalBinary(binaryPath), name: 'local-binary', requiresChecksum: false },
        { method: () => this.installFromDownload(binaryPath), name: 'download', requiresChecksum: true }
      ];

      let installed = false;
      let lastError;
      let usedMethod = null;

      for (const { method, name, requiresChecksum } of installMethods) {
        try {
          await method();
          installed = true;
          usedMethod = { name, requiresChecksum };
          break;
        } catch (error) {
          lastError = error;
          console.log(`⚠️  Installation method failed: ${error.message}`);
        }
      }

      if (!installed) {
        throw lastError || new Error('All installation methods failed');
      }

      // Make binary executable
      await this.makeExecutable(binaryPath);

      // Verify binary integrity with checksum (primarily for downloaded binaries)
      if (usedMethod && usedMethod.requiresChecksum) {
        console.log('🔐 Verifying downloaded binary integrity...');
        const isValidChecksum = await this.verifyBinaryChecksum(binaryPath);
        if (!isValidChecksum) {
          await this.cleanup([binaryPath]);
          throw new Error('Binary installation failed - checksum verification failed. The binary may have been tampered with or corrupted during download.');
        }
      } else if (usedMethod) {
        console.log(`ℹ️  Using ${usedMethod.name} binary - checksum verification not required for local sources`);
      }

      // Test the binary
      if (await this.testBinary(binaryPath)) {
        const duration = Date.now() - this.installStartTime;
        console.log(`✅ LSP Gateway installed successfully! (${duration}ms)`);
        console.log('');
        await this.showQuickStart();
        await this.showPostInstallInfo();
      } else {
        throw new Error('Binary installation failed - binary is not working properly');
      }

    } catch (error) {
      await this.handleInstallationError(error);
    } finally {
      await this.cleanup(this.tempFiles);
    }
  }

  /**
   * Install from build output directory
   * @param {string} binaryPath - Destination binary path
   * @returns {Promise<void>}
   */
  async installFromBuildOutput(binaryPath) {
    const sourcePath = this.platform.getSourceBinaryPath();
    if (await this.fileExists(sourcePath)) {
      console.log('🔧 Using build output binary...');
      await copyFile(sourcePath, binaryPath);
      return;
    }
    throw new Error('Build output binary not found');
  }

  /**
   * Install from local development binary
   * @param {string} binaryPath - Destination binary path
   * @returns {Promise<void>}
   */
  async installFromLocalBinary(binaryPath) {
    const localPath = this.platform.getLocalBinaryPath();
    if (await this.fileExists(localPath)) {
      console.log('🔧 Using local development binary...');
      await copyFile(localPath, binaryPath);
      return;
    }
    throw new Error('Local development binary not found');
  }

  /**
   * Install from download
   * @param {string} binaryPath - Destination binary path
   * @returns {Promise<void>}
   */
  async installFromDownload(binaryPath) {
    console.log('📥 Downloading binary...');
    await this.downloadBinary(binaryPath);
  }

  /**
   * Download binary from GitHub releases
   * @param {string} binaryPath - Path where to save the binary
   * @returns {Promise<void>}
   */
  async downloadBinary(binaryPath) {
    const url = this.platform.getDownloadUrl(this.version);
    console.log(`📥 Downloading from: ${url}`);
    
    // Add to temp files for cleanup
    this.tempFiles.push(binaryPath);
    
    return new Promise((resolve, reject) => {
      const file = fs.createWriteStream(binaryPath);
      let downloadStartTime = Date.now();
      
      const request = https.get(url, (response) => {
        // Handle redirects
        if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
          console.log(`📍 Redirecting to: ${response.headers.location}`);
          file.close();
          https.get(response.headers.location, (redirectResponse) => {
            if (redirectResponse.statusCode === 200) {
              const newFile = fs.createWriteStream(binaryPath);
              this.setupDownloadProgress(redirectResponse, newFile, downloadStartTime);
              redirectResponse.pipe(newFile);
              newFile.on('finish', () => {
                console.log('\n✅ Download completed');
                newFile.close();
                resolve();
              });
              newFile.on('error', (err) => {
                this.cleanup([binaryPath]);
                reject(new Error(`File write error: ${err.message}`));
              });
            } else {
              reject(new Error(`Download failed with status: ${redirectResponse.statusCode}`));
            }
          }).on('error', reject);
          return;
        }
        
        if (response.statusCode === 200) {
          this.setupDownloadProgress(response, file, downloadStartTime);
          response.pipe(file);
        } else if (response.statusCode === 404) {
          reject(new Error(`Binary not found for version ${this.version}. Please check if the release exists.`));
        } else {
          reject(new Error(`Download failed with status: ${response.statusCode}`));
        }
      });
      
      request.setTimeout(30000, () => {
        request.abort();
        reject(new Error('Download request timed out'));
      });
      
      request.on('error', (err) => {
        reject(new Error(`Download request failed: ${err.message}`));
      });
      
      file.on('finish', () => {
        console.log('\n✅ Download completed');
        file.close();
        // Remove from temp files since download succeeded
        this.tempFiles = this.tempFiles.filter(f => f !== binaryPath);
        resolve();
      });
      
      file.on('error', (err) => {
        this.cleanup([binaryPath]);
        reject(new Error(`File write error: ${err.message}`));
      });
    });
  }

  /**
   * Setup download progress reporting
   * @param {http.IncomingMessage} response - HTTP response
   * @param {fs.WriteStream} file - File stream
   * @param {number} startTime - Download start time
   */
  setupDownloadProgress(response, file, startTime) {
    let downloadedBytes = 0;
    const totalBytes = parseInt(response.headers['content-length'] || '0');
    let lastUpdate = 0;
    
    response.on('data', (chunk) => {
      downloadedBytes += chunk.length;
      const now = Date.now();
      
      // Update progress every 200ms to avoid spam
      if (now - lastUpdate > 200) {
        lastUpdate = now;
        if (totalBytes > 0) {
          const progress = Math.round((downloadedBytes / totalBytes) * 100);
          const elapsed = (now - startTime) / 1000;
          const speed = downloadedBytes / elapsed;
          const speedStr = this.formatBytes(speed);
          const totalStr = this.formatBytes(totalBytes);
          const downloadedStr = this.formatBytes(downloadedBytes);
          
          process.stdout.write(`\r📥 Downloading... ${progress}% (${downloadedStr}/${totalStr}) @ ${speedStr}/s`);
        } else {
          const downloadedStr = this.formatBytes(downloadedBytes);
          process.stdout.write(`\r📥 Downloading... ${downloadedStr}`);
        }
      }
    });
  }

  /**
   * Format bytes for human-readable display
   * @param {number} bytes - Number of bytes
   * @returns {string} Formatted string
   */
  formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  /**
   * Make binary executable
   * @param {string} binaryPath - Path to the binary
   * @returns {Promise<void>}
   */
  async makeExecutable(binaryPath) {
    if (this.platform.isWindows()) {
      // Windows doesn't need chmod
      return;
    }
    
    try {
      await chmod(binaryPath, 0o755);
      console.log('✅ Binary made executable');
    } catch (error) {
      throw new Error(`Failed to make binary executable: ${error.message}`);
    }
  }

  /**
   * Test if the binary works
   * @param {string} binaryPath - Path to the binary
   * @returns {Promise<boolean>}
   */
  async testBinary(binaryPath) {
    return new Promise((resolve) => {
      const child = spawn(binaryPath, ['version'], {
        stdio: 'pipe',
        timeout: 10000
      });
      
      let output = '';
      let errorOutput = '';
      
      child.stdout.on('data', (data) => {
        output += data.toString();
      });
      
      child.stderr.on('data', (data) => {
        errorOutput += data.toString();
      });
      
      child.on('close', (code) => {
        // Check if command executed successfully and output contains version info
        const success = code === 0 && (output.includes('lspg') || output.includes('version') || output.includes('dev') || output.trim().length > 0);
        if (success) {
          console.log('✅ Binary test successful');
        } else {
          console.log(`⚠️  Binary test failed: code=${code}, output="${output}", error="${errorOutput}"`);
        }
        resolve(success);
      });
      
      child.on('error', (error) => {
        console.log(`⚠️  Binary test error: ${error.message}`);
        resolve(false);
      });
    });
  }

  /**
   * Show quick start information
   */
  async showQuickStart() {
    console.log('🚀 Quick Start:');
    console.log('  lspg server              # Start with default config');
    console.log('  lspg --help              # Show help');
    console.log('  npm run check-servers           # Check LSP server availability');
    console.log('  npm run create-config           # Create default config');
    console.log('');
    console.log('📚 Documentation: https://github.com/lsp-gateway/lsp-gateway');
    console.log('');
  }

  /**
   * Show post-installation information and recommendations
   */
  async showPostInstallInfo() {
    console.log('📝 Next Steps:');
    console.log('');
    console.log('1. Install Language Servers (as needed):');
    console.log('   • Go: go install golang.org/x/tools/gopls@latest');
    console.log('   • Python: pip install python-lsp-server');
    console.log('   • TypeScript/JavaScript: npm install -g typescript-language-server');
    console.log('   • Java: Download Eclipse JDT Language Server');
    console.log('');
    console.log('2. Create configuration file:');
    console.log('   npm run create-config');
    console.log('');
    console.log('3. Check LSP server availability:');
    console.log('   npm run check-servers');
    console.log('');
    console.log('4. Start the gateway:');
    console.log('   lspg server');
    console.log('');
    console.log('💡 Tips:');
    console.log('   • Use lspg --help for all available options');
    console.log('   • The server runs on port 8080 by default');
    console.log('   • LSP requests are sent to http://localhost:8080/jsonrpc');
    console.log('   • Supports Go, Python, TypeScript, JavaScript, and Java');
  }

  /**
   * Handle installation errors with comprehensive troubleshooting
   * @param {Error} error - The error that occurred
   */
  async handleInstallationError(error) {
    console.error('❌ Installation failed:', error.message);
    console.error('');
    console.error('📝 Troubleshooting:');
    console.error('  1. Check that your platform is supported');
    console.error('  2. Ensure you have internet connectivity');
    console.error('  3. Try running: npm install --force');
    console.error('  4. Check the GitHub releases page for manual download');
    console.error('  5. Verify file permissions in the installation directory');
    console.error('');
    console.error('Platform Info:');
    console.error(`  Current: ${this.platform.getCurrentPlatformString()}`);
    console.error(`  Supported: ${this.platform.getSupportedPlatforms().join(', ')}`);
    console.error('');
    console.error('Environment:');
    console.error(`  Node.js: ${process.version}`);
    console.error(`  OS: ${process.platform} ${process.arch}`);
    console.error(`  Working Directory: ${process.cwd()}`);
    console.error('');
    
    // Try to provide specific help based on the error
    if (error.message.includes('EACCES') || error.message.includes('permission')) {
      console.error('💡 Permission Error Solutions:');
      console.error('  • Run with sudo (Linux/macOS): sudo npm install');
      console.error('  • Run as Administrator (Windows)');
      console.error('  • Check directory permissions');
      console.error('');
    }
    
    if (error.message.includes('ENOTFOUND') || error.message.includes('network')) {
      console.error('💡 Network Error Solutions:');
      console.error('  • Check your internet connection');
      console.error('  • Try using a different network');
      console.error('  • Check if GitHub is accessible');
      console.error('');
    }
    
    if (error.message.includes('checksum verification failed')) {
      console.error('🔐 Security Error - Checksum Verification Failed:');
      console.error('  • The downloaded binary failed integrity verification');
      console.error('  • This could indicate a man-in-the-middle attack or corrupted download');
      console.error('  • Try downloading again from a different network');
      console.error('  • Verify your network connection is secure (use HTTPS)');
      console.error('  • Contact the maintainers if the problem persists');
      console.error('  • For development only: SKIP_CHECKSUM_VERIFICATION=true (NOT RECOMMENDED)');
      console.error('');
    }
    
    // Don't fail the installation completely - allow manual setup
    console.log('⚠️  You can manually place the binary in the bin/ directory');
    console.log('   or build from source using: make build');
    process.exit(0);
  }

  /**
   * Ensure a directory exists
   * @param {string} dirPath - Directory path
   */
  async ensureDirectory(dirPath) {
    try {
      await mkdir(dirPath, { recursive: true });
    } catch (error) {
      if (error.code !== 'EEXIST') {
        throw error;
      }
    }
  }

  /**
   * Check if a file exists
   * @param {string} filePath - File path
   * @returns {Promise<boolean>}
   */
  async fileExists(filePath) {
    try {
      await access(filePath);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Cleanup temporary files
   * @param {Array<string>} files - Array of file paths to cleanup
   */
  async cleanup(files) {
    if (!files || files.length === 0) return;
    
    for (const file of files) {
      try {
        if (await this.fileExists(file)) {
          await unlink(file);
        }
      } catch (error) {
        // Ignore cleanup errors
      }
    }
  }
}

// Run installer if this script is executed directly
if (require.main === module) {
  const installer = new Installer();
  
  // Handle process termination gracefully
  process.on('SIGINT', async () => {
    console.log('\n⚠️  Installation interrupted');
    await installer.cleanup(installer.tempFiles);
    process.exit(1);
  });
  
  process.on('SIGTERM', async () => {
    console.log('\n⚠️  Installation terminated');
    await installer.cleanup(installer.tempFiles);
    process.exit(1);
  });
  
  installer.install().catch(async (error) => {
    console.error('Installation failed:', error);
    await installer.cleanup(installer.tempFiles);
    process.exit(1);
  });
}

module.exports = Installer;