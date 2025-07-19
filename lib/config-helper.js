const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Configuration helper for LSP Gateway
 * Provides utilities for managing configuration files
 */

class ConfigHelper {
  /**
   * Create a default configuration file
   * @param {string} filePath - Path where to create the config file
   * @returns {Promise<void>}
   */
  static async createDefaultConfig(filePath) {
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

  /**
   * Check if required LSP servers are available
   * @returns {Promise<Object>} Availability status for each server
   */
  static async checkLSPServers() {
    const servers = [
      { name: 'gopls', command: 'gopls', args: ['version'] },
      { name: 'pylsp', command: 'python', args: ['-m', 'pylsp', '--version'] },
      { name: 'typescript-language-server', command: 'typescript-language-server', args: ['--version'] },
      { name: 'jdtls', command: 'jdtls', args: ['--version'] }
    ];

    const results = {};
    
    for (const server of servers) {
      try {
        const available = await this.checkCommand(server.command, server.args);
        results[server.name] = {
          available,
          command: server.command,
          status: available ? 'Available' : 'Not found'
        };
      } catch (error) {
        results[server.name] = {
          available: false,
          command: server.command,
          status: `Error: ${error.message}`
        };
      }
    }
    
    return results;
  }

  /**
   * Check if a command is available
   * @param {string} command - Command to check
   * @param {Array} args - Arguments to pass
   * @returns {Promise<boolean>} True if command is available
   */
  static async checkCommand(command, args = []) {
    return new Promise((resolve) => {
      const child = spawn(command, args, {
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
   * Validate configuration file
   * @param {string} configPath - Path to configuration file
   * @returns {Promise<Object>} Validation result
   */
  static async validateConfig(configPath) {
    try {
      if (!fs.existsSync(configPath)) {
        return {
          valid: false,
          error: 'Configuration file not found'
        };
      }

      const content = fs.readFileSync(configPath, 'utf8');
      
      // Basic YAML validation (simplified)
      if (!content.includes('port:') || !content.includes('servers:')) {
        return {
          valid: false,
          error: 'Invalid configuration format'
        };
      }

      return {
        valid: true,
        message: 'Configuration file is valid'
      };
    } catch (error) {
      return {
        valid: false,
        error: error.message
      };
    }
  }

  /**
   * Get configuration recommendations
   * @returns {Object} Configuration recommendations
   */
  static getRecommendations() {
    return {
      recommendations: [
        'Install gopls for Go support: go install golang.org/x/tools/gopls@latest',
        'Install pylsp for Python support: pip install python-lsp-server',
        'Install typescript-language-server for TypeScript/JavaScript support: npm install -g typescript-language-server',
        'Install jdtls for Java support: Download from Eclipse JDT Language Server'
      ],
      ports: {
        default: 8080,
        alternatives: [8081, 8082, 3000, 3001]
      },
      transports: {
        supported: ['stdio'],
        planned: ['tcp', 'websocket']
      }
    };
  }
}

module.exports = ConfigHelper;