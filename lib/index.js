const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const PlatformInfo = require('./platform');
const Installer = require('./installer');

/**
 * LSP Gateway NPM Package
 * Main entry point for programmatic access to LSP Gateway
 */

class LSPGateway {
  constructor(options = {}) {
    this.platform = new PlatformInfo();
    this.options = {
      port: options.port || 8080,
      config: options.config || path.join(__dirname, '..', 'config.yaml'),
      ...options
    };
    this.process = null;
    this.isRunning = false;
  }

  /**
   * Get the path to the binary
   * @returns {string} Binary path
   */
  getBinaryPath() {
    return this.platform.getBinaryPath();
  }

  /**
   * Check if the binary exists and is executable
   * @returns {Promise<boolean>} True if binary is ready
   */
  async isBinaryReady() {
    const binaryPath = this.getBinaryPath();
    
    if (!fs.existsSync(binaryPath)) {
      return false;
    }

    // Test if binary works
    return new Promise((resolve) => {
      const child = spawn(binaryPath, ['--version'], {
        stdio: 'pipe',
        timeout: 5000
      });
      
      child.on('close', (code) => {
        resolve(code === 0);
      });
      
      child.on('error', () => {
        resolve(false);
      });
    });
  }

  /**
   * Install the binary if it's not available
   * @returns {Promise<void>}
   */
  async ensureBinary() {
    if (!(await this.isBinaryReady())) {
      console.log('🔧 Binary not found, installing...');
      const installer = new Installer();
      await installer.install();
    }
  }

  /**
   * Start the LSP Gateway server
   * @param {Object} options - Server options
   * @returns {Promise<void>}
   */
  async start(options = {}) {
    if (this.isRunning) {
      throw new Error('LSP Gateway is already running');
    }

    await this.ensureBinary();
    
    const binaryPath = this.getBinaryPath();
    const args = ['server'];
    
    // Add configuration file
    if (options.config || this.options.config) {
      args.push('--config', options.config || this.options.config);
    }
    
    // Add port
    if (options.port || this.options.port) {
      args.push('--port', (options.port || this.options.port).toString());
    }

    console.log(`🚀 Starting LSP Gateway on port ${options.port || this.options.port}...`);
    
    return new Promise((resolve, reject) => {
      this.process = spawn(binaryPath, args, {
        stdio: 'inherit'
      });
      
      this.process.on('spawn', () => {
        this.isRunning = true;
        console.log('✅ LSP Gateway started successfully');
        resolve();
      });
      
      this.process.on('error', (error) => {
        this.isRunning = false;
        reject(new Error(`Failed to start LSP Gateway: ${error.message}`));
      });
      
      this.process.on('close', (code) => {
        this.isRunning = false;
        if (code !== 0) {
          console.log(`⚠️  LSP Gateway exited with code ${code}`);
        }
      });
    });
  }

  /**
   * Stop the LSP Gateway server
   * @returns {Promise<void>}
   */
  async stop() {
    if (!this.isRunning || !this.process) {
      return;
    }
    
    return new Promise((resolve) => {
      this.process.on('close', () => {
        this.isRunning = false;
        this.process = null;
        console.log('🛑 LSP Gateway stopped');
        resolve();
      });
      
      // Send SIGTERM
      this.process.kill('SIGTERM');
      
      // Force kill after 5 seconds
      setTimeout(() => {
        if (this.process) {
          this.process.kill('SIGKILL');
        }
      }, 5000);
    });
  }

  /**
   * Get the status of the LSP Gateway
   * @returns {Object} Status information
   */
  getStatus() {
    return {
      isRunning: this.isRunning,
      port: this.options.port,
      config: this.options.config,
      platform: this.platform.getPlatformName(),
      binaryPath: this.getBinaryPath()
    };
  }

  /**
   * Get platform information
   * @returns {Object} Platform information
   */
  getPlatformInfo() {
    return {
      platform: this.platform.getCurrentPlatform(),
      name: this.platform.getPlatformName(),
      supported: this.platform.isSupported(),
      binaryPath: this.getBinaryPath()
    };
  }

  /**
   * Create a default configuration file
   * @param {string} filePath - Path where to create the config file
   * @returns {Promise<void>}
   */
  async createDefaultConfig(filePath) {
    const defaultConfig = `port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
    
  - name: "python-lsp"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    
  - name: "typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
    
  - name: "java-lsp"
    languages: ["java"]
    command: "jdtls"
    args: []
    transport: "stdio"
`;

    return new Promise((resolve, reject) => {
      fs.writeFile(filePath, defaultConfig, 'utf8', (err) => {
        if (err) {
          reject(new Error(`Failed to create config file: ${err.message}`));
        } else {
          console.log(`✅ Default configuration created at: ${filePath}`);
          resolve();
        }
      });
    });
  }
}

// Export the main class and utilities
module.exports = {
  LSPGateway,
  PlatformInfo,
  Installer,
  
  // Convenience methods
  async createGateway(options) {
    return new LSPGateway(options);
  },
  
  async getPlatformInfo() {
    const platform = new PlatformInfo();
    return platform.getPlatformInfo();
  },
  
  async installBinary() {
    const installer = new Installer();
    return installer.install();
  }
};