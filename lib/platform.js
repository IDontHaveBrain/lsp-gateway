const os = require('os');
const path = require('path');

/**
 * Platform detection and binary information for LSP Gateway
 * Supports Linux x64, Windows x64, macOS x64 (Intel), and macOS arm64 (Apple Silicon)
 */

class PlatformInfo {
  constructor() {
    this.platform = os.platform();
    this.arch = os.arch();
    this.supportedPlatforms = {
      'linux-x64': {
        platform: 'linux',
        arch: 'x64',
        binaryName: 'lsp-gateway-linux',
        executable: 'lsp-gateway',
        downloadUrl: 'https://github.com/IDontHaveBrain/lsp-gateway/releases/download/{version}/lsp-gateway-linux'
      },
      'win32-x64': {
        platform: 'win32',
        arch: 'x64', 
        binaryName: 'lsp-gateway-windows.exe',
        executable: 'lsp-gateway.exe',
        downloadUrl: 'https://github.com/IDontHaveBrain/lsp-gateway/releases/download/{version}/lsp-gateway-windows.exe'
      },
      'darwin-x64': {
        platform: 'darwin',
        arch: 'x64',
        binaryName: 'lsp-gateway-macos',
        executable: 'lsp-gateway',
        downloadUrl: 'https://github.com/IDontHaveBrain/lsp-gateway/releases/download/{version}/lsp-gateway-macos'
      },
      'darwin-arm64': {
        platform: 'darwin',
        arch: 'arm64',
        binaryName: 'lsp-gateway-macos-arm64',
        executable: 'lsp-gateway',
        downloadUrl: 'https://github.com/IDontHaveBrain/lsp-gateway/releases/download/{version}/lsp-gateway-macos-arm64'
      }
    };
  }

  /**
   * Get the current platform identifier
   * @returns {Object} Platform object with os and arch properties
   */
  getCurrentPlatform() {
    // Handle architecture aliases
    let arch = this.arch;
    if (arch === 'x64' || arch === 'amd64') {
      arch = 'x64';
    } else if (arch === 'arm64' || arch === 'aarch64') {
      arch = 'arm64';
    } else {
      throw new Error(`Unsupported architecture: ${this.arch}`);
    }
    
    // Validate platform combinations
    if (this.platform === 'linux' && arch === 'x64') {
      return { os: 'linux', arch: 'x64' };
    } else if (this.platform === 'win32' && arch === 'x64') {
      return { os: 'win32', arch: 'x64' };
    } else if (this.platform === 'darwin' && (arch === 'x64' || arch === 'arm64')) {
      return { os: 'darwin', arch };
    }
    
    throw new Error(`Unsupported platform combination: ${this.platform}-${this.arch}`);
  }

  /**
   * Get the current platform identifier as string (legacy method)
   * @returns {string} Platform identifier (e.g., 'linux-x64', 'darwin-arm64')
   */
  getCurrentPlatformString() {
    const current = this.getCurrentPlatform();
    return `${current.os}-${current.arch}`;
  }

  /**
   * Check if the current platform is supported
   * @returns {boolean} True if platform is supported
   */
  isSupported() {
    try {
      const current = this.getCurrentPlatformString();
      return current in this.supportedPlatforms;
    } catch (error) {
      return false;
    }
  }

  /**
   * Get platform information for the current system
   * @returns {Object} Platform information with platform, name, and supported properties
   */
  getPlatformInfo() {
    const currentPlatform = this.getCurrentPlatform();
    const supported = this.isSupported();
    
    if (!supported) {
      throw new Error(`Platform ${this.platform}-${this.arch} is not supported. Supported platforms: ${Object.keys(this.supportedPlatforms).join(', ')}`);
    }
    
    return {
      platform: `${currentPlatform.os}-${currentPlatform.arch}`,
      name: this.getPlatformName(),
      supported: supported
    };
  }

  /**
   * Get platform configuration object (legacy method)
   * @returns {Object} Platform configuration object with binary info
   */
  getPlatformConfig() {
    if (!this.isSupported()) {
      throw new Error(`Platform ${this.platform}-${this.arch} is not supported. Supported platforms: ${Object.keys(this.supportedPlatforms).join(', ')}`);
    }
    
    const current = this.getCurrentPlatformString();
    return this.supportedPlatforms[current];
  }

  /**
   * Get the binary path for the current platform
   * @returns {string} Path to the binary
   */
  getBinaryPath() {
    const info = this.getPlatformConfig();
    return path.join(__dirname, '..', 'bin', info.executable);
  }

  /**
   * Get the source binary path (from build output)
   * @returns {string} Path to the source binary
   */
  getSourceBinaryPath() {
    const info = this.getPlatformConfig();
    return path.join(__dirname, '..', 'bin', info.binaryName);
  }

  /**
   * Get the local development binary path
   * @returns {string} Path to the local binary
   */
  getLocalBinaryPath() {
    return path.join(__dirname, '..', 'lsp-gateway');
  }

  /**
   * Get the download URL for a specific version
   * @param {string} version - Version to download
   * @returns {string} Download URL
   */
  getDownloadUrl(version) {
    const info = this.getPlatformConfig();
    // Ensure version starts with 'v' prefix
    const versionString = version.startsWith('v') ? version : `v${version}`;
    return info.downloadUrl.replace('{version}', versionString);
  }

  /**
   * Get all supported platforms
   * @returns {Array} Array of supported platform identifiers
   */
  getSupportedPlatforms() {
    return Object.keys(this.supportedPlatforms);
  }

  /**
   * Check if we're on Windows
   * @returns {boolean} True if on Windows
   */
  isWindows() {
    return this.platform === 'win32';
  }

  /**
   * Check if we're on macOS
   * @returns {boolean} True if on macOS
   */
  isMacOS() {
    return this.platform === 'darwin';
  }

  /**
   * Check if we're on Linux
   * @returns {boolean} True if on Linux
   */
  isLinux() {
    return this.platform === 'linux';
  }

  /**
   * Check if we're on Apple Silicon
   * @returns {boolean} True if on Apple Silicon
   */
  isAppleSilicon() {
    return this.platform === 'darwin' && this.arch === 'arm64';
  }

  /**
   * Get user-friendly platform name
   * @returns {string} User-friendly platform name
   */
  getPlatformName() {
    const current = this.getCurrentPlatformString();
    const names = {
      'linux-x64': 'Linux x64',
      'win32-x64': 'Windows x64',
      'darwin-x64': 'macOS Intel',
      'darwin-arm64': 'macOS Apple Silicon'
    };
    return names[current] || current;
  }
}

module.exports = PlatformInfo;