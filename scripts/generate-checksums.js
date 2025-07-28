#!/usr/bin/env node

/**
 * Checksum generation utility for LSP Gateway releases
 * This script helps maintainers generate SHA-256 checksums for binary releases
 */

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

/**
 * Calculate SHA-256 checksum of a file
 * @param {string} filePath - Path to the file
 * @returns {Promise<string>} SHA-256 hash in hexadecimal
 */
async function calculateChecksum(filePath) {
  return new Promise((resolve, reject) => {
    if (!fs.existsSync(filePath)) {
      reject(new Error(`File not found: ${filePath}`));
      return;
    }

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
 * Generate checksums for all platform binaries
 */
async function generateChecksums() {
  const version = process.argv[2];
  if (!version) {
    console.error('Usage: node generate-checksums.js <version>');
    console.error('Example: node generate-checksums.js v1.0.0');
    process.exit(1);
  }

  console.log(`🔐 Generating SHA-256 checksums for LSP Gateway ${version}...\n`);

  const platforms = [
    { key: 'linux-x64', filename: 'lspg-linux' },
    { key: 'win32-x64', filename: 'lspg-windows.exe' },
    { key: 'darwin-x64', filename: 'lspg-macos' },
    { key: 'darwin-arm64', filename: 'lspg-macos-arm64' }
  ];

  const binDir = path.join(__dirname, '..', 'bin');
  const checksums = {};
  let foundBinaries = 0;

  console.log('Calculating checksums for binaries in bin/ directory:\n');

  for (const platform of platforms) {
    const binaryPath = path.join(binDir, platform.filename);
    
    try {
      const checksum = await calculateChecksum(binaryPath);
      checksums[platform.key] = checksum;
      foundBinaries++;
      
      console.log(`✅ ${platform.key.padEnd(12)}: ${checksum}`);
      console.log(`   File: ${platform.filename}\n`);
    } catch (error) {
      console.log(`⚠️  ${platform.key.padEnd(12)}: ${error.message}\n`);
    }
  }

  if (foundBinaries === 0) {
    console.error('❌ No binaries found in bin/ directory');
    console.error('   Please build the binaries first using: make build');
    process.exit(1);
  }

  // Generate JavaScript object for insertion into installer.js
  console.log('='.repeat(60));
  console.log(`\n📋 Checksum data for installer.js (${foundBinaries}/${platforms.length} platforms):\n`);
  
  console.log(`'${version}': {`);
  for (const [platformKey, checksum] of Object.entries(checksums)) {
    console.log(`  '${platformKey}': '${checksum}',`);
  }
  console.log('}');

  console.log('\n📝 Instructions:');
  console.log('1. Copy the checksum data above');
  console.log('2. Add it to the checksums object in lib/installer.js');
  console.log('3. Update any existing placeholder checksums');
  console.log('4. Test with: npm install --force');
  console.log('5. Commit the changes with the release');

  if (foundBinaries < platforms.length) {
    console.log('\n⚠️  Missing binaries for some platforms:');
    for (const platform of platforms) {
      if (!checksums[platform.key]) {
        console.log(`   - ${platform.key}: ${platform.filename}`);
      }
    }
    console.log('   Build missing binaries with: make build');
  }

  console.log('\n🔐 Security Note:');
  console.log('   These checksums will be used to verify binary integrity during installation');
  console.log('   Ensure they are generated from trusted, official build artifacts');
}

if (require.main === module) {
  generateChecksums().catch(error => {
    console.error('Checksum generation failed:', error);
    process.exit(1);
  });
}

module.exports = { calculateChecksum, generateChecksums };