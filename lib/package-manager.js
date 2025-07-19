#!/usr/bin/env node

/**
 * Package management utilities for LSP Gateway
 * Provides commands for maintaining and managing the NPM package
 */

const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const PlatformInfo = require('./platform');
const ConfigHelper = require('./config-helper');

class PackageManager {
  constructor() {
    this.platform = new PlatformInfo();
    this.packageJson = require('../package.json');
    this.version = this.packageJson.version;
  }

  /**
   * Show package status information
   */
  async showStatus() {
    console.log('📦 LSP Gateway Package Status');
    console.log('');
    console.log(`Version: ${this.version}`);
    console.log(`Platform: ${this.platform.getPlatformName()}`);
    console.log(`Binary Path: ${this.platform.getBinaryPath()}`);
    console.log(`Supported: ${this.platform.isSupported()}`);
    console.log('');

    // Check binary existence
    const binaryPath = this.platform.getBinaryPath();
    if (fs.existsSync(binaryPath)) {
      console.log('✅ Binary exists');
      
      // Test binary
      try {
        const works = await this.testBinary(binaryPath);
        console.log(`${works ? '✅' : '❌'} Binary is ${works ? 'working' : 'not working'}`);
      } catch (error) {
        console.log(`❌ Binary test failed: ${error.message}`);
      }
    } else {
      console.log('❌ Binary not found');
    }

    // Check configuration
    const configPath = path.join(process.cwd(), 'config.yaml');
    if (fs.existsSync(configPath)) {
      console.log('✅ Configuration file exists');
      
      try {
        const validation = await ConfigHelper.validateConfig(configPath);
        console.log(`${validation.valid ? '✅' : '❌'} Configuration is ${validation.valid ? 'valid' : 'invalid'}`);
        if (!validation.valid) {
          console.log(`   Error: ${validation.error}`);
        }
      } catch (error) {
        console.log(`❌ Configuration validation failed: ${error.message}`);
      }
    } else {
      console.log('⚠️  No configuration file found');
    }
  }

  /**
   * Clean up package files
   */
  async cleanup() {
    console.log('🧹 Cleaning up LSP Gateway package...');
    
    const binDir = path.join(__dirname, '..', 'bin');
    const binaryPath = this.platform.getBinaryPath();
    
    if (fs.existsSync(binaryPath)) {
      try {
        fs.unlinkSync(binaryPath);
        console.log('✅ Removed binary');
      } catch (error) {
        console.log(`❌ Failed to remove binary: ${error.message}`);
      }
    }

    console.log('✅ Cleanup completed');
  }

  /**
   * Check LSP server availability
   */
  async checkServers() {
    console.log('🔍 Checking LSP server availability...');
    console.log('');
    
    try {
      const results = await ConfigHelper.checkLSPServers();
      
      for (const [name, info] of Object.entries(results)) {
        const icon = info.available ? '✅' : '❌';
        console.log(`${icon} ${name}: ${info.status}`);
        if (!info.available) {
          console.log(`   Command: ${info.command}`);
        }
      }
    } catch (error) {
      console.log(`❌ Server check failed: ${error.message}`);
    }
  }

  /**
   * Show installation recommendations
   */
  showRecommendations() {
    console.log('💡 LSP Gateway Recommendations');
    console.log('');
    
    const recommendations = ConfigHelper.getRecommendations();
    
    console.log('📚 Language Server Installation:');
    recommendations.recommendations.forEach((rec, index) => {
      console.log(`  ${index + 1}. ${rec}`);
    });
    
    console.log('');
    console.log('🔧 Configuration:');
    console.log(`  • Default port: ${recommendations.ports.default}`);
    console.log(`  • Alternative ports: ${recommendations.ports.alternatives.join(', ')}`);
    console.log(`  • Supported transports: ${recommendations.transports.supported.join(', ')}`);
    console.log(`  • Planned transports: ${recommendations.transports.planned.join(', ')}`);
  }

  /**
   * Test if binary works
   */
  async testBinary(binaryPath) {
    return new Promise((resolve) => {
      const child = spawn(binaryPath, ['--version'], {
        stdio: 'pipe',
        timeout: 5000
      });
      
      let output = '';
      child.stdout.on('data', (data) => {
        output += data.toString();
      });
      
      child.on('close', (code) => {
        resolve(code === 0 && output.includes('lsp-gateway'));
      });
      
      child.on('error', () => {
        resolve(false);
      });
    });
  }

  /**
   * Show package information
   */
  showInfo() {
    console.log('📋 LSP Gateway Package Information');
    console.log('');
    console.log(`Name: ${this.packageJson.name}`);
    console.log(`Version: ${this.packageJson.version}`);
    console.log(`Description: ${this.packageJson.description}`);
    console.log(`Author: ${this.packageJson.author}`);
    console.log(`License: ${this.packageJson.license}`);
    console.log(`Homepage: ${this.packageJson.homepage}`);
    console.log(`Repository: ${this.packageJson.repository.url}`);
    console.log('');
    console.log('Supported Platforms:');
    this.platform.getSupportedPlatforms().forEach(platform => {
      console.log(`  • ${platform}`);
    });
    console.log('');
    console.log('Supported Languages:');
    console.log('  • Go (gopls)');
    console.log('  • Python (pylsp)');
    console.log('  • TypeScript/JavaScript (typescript-language-server)');
    console.log('  • Java (jdtls)');
  }

  /**
   * Run diagnostics
   */
  async runDiagnostics() {
    console.log('🔬 Running LSP Gateway Diagnostics');
    console.log('');
    
    // Platform check
    console.log('1. Platform Support:');
    console.log(`   Current: ${this.platform.getCurrentPlatform()}`);
    console.log(`   Supported: ${this.platform.isSupported()}`);
    console.log('');
    
    // Binary check
    console.log('2. Binary Status:');
    const binaryPath = this.platform.getBinaryPath();
    const binaryExists = fs.existsSync(binaryPath);
    console.log(`   Exists: ${binaryExists}`);
    
    if (binaryExists) {
      try {
        const stats = fs.statSync(binaryPath);
        console.log(`   Size: ${(stats.size / 1024 / 1024).toFixed(2)} MB`);
        console.log(`   Modified: ${stats.mtime.toISOString()}`);
        
        // Check if executable
        try {
          fs.accessSync(binaryPath, fs.constants.X_OK);
          console.log('   Executable: Yes');
        } catch {
          console.log('   Executable: No');
        }
        
        // Test binary
        const works = await this.testBinary(binaryPath);
        console.log(`   Working: ${works}`);
      } catch (error) {
        console.log(`   Error: ${error.message}`);
      }
    }
    
    console.log('');
    
    // Configuration check
    console.log('3. Configuration:');
    const configPath = path.join(process.cwd(), 'config.yaml');
    const configExists = fs.existsSync(configPath);
    console.log(`   Config exists: ${configExists}`);
    
    if (configExists) {
      try {
        const validation = await ConfigHelper.validateConfig(configPath);
        console.log(`   Valid: ${validation.valid}`);
        if (!validation.valid) {
          console.log(`   Error: ${validation.error}`);
        }
      } catch (error) {
        console.log(`   Validation error: ${error.message}`);
      }
    }
    
    console.log('');
    
    // LSP servers check
    console.log('4. LSP Servers:');
    try {
      const results = await ConfigHelper.checkLSPServers();
      for (const [name, info] of Object.entries(results)) {
        console.log(`   ${name}: ${info.available ? 'Available' : 'Not found'}`);
      }
    } catch (error) {
      console.log(`   Error: ${error.message}`);
    }
  }
}

// CLI interface
if (require.main === module) {
  const manager = new PackageManager();
  const command = process.argv[2];
  
  (async () => {
    switch (command) {
      case 'status':
        await manager.showStatus();
        break;
      case 'cleanup':
        await manager.cleanup();
        break;
      case 'check-servers':
        await manager.checkServers();
        break;
      case 'recommendations':
        manager.showRecommendations();
        break;
      case 'info':
        manager.showInfo();
        break;
      case 'diagnostics':
        await manager.runDiagnostics();
        break;
      default:
        console.log('📦 LSP Gateway Package Manager');
        console.log('');
        console.log('Usage: node lib/package-manager.js <command>');
        console.log('');
        console.log('Commands:');
        console.log('  status           Show package status');
        console.log('  cleanup          Clean up package files');
        console.log('  check-servers    Check LSP server availability');
        console.log('  recommendations  Show setup recommendations');
        console.log('  info             Show package information');
        console.log('  diagnostics      Run comprehensive diagnostics');
        console.log('');
        console.log('Examples:');
        console.log('  npm run status');
        console.log('  node lib/package-manager.js diagnostics');
        break;
    }
  })().catch(error => {
    console.error('❌ Error:', error.message);
    process.exit(1);
  });
}

module.exports = PackageManager;