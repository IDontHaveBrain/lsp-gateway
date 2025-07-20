#!/usr/bin/env node

/**
 * Test Framework Validation Script
 * Validates that the runtime detection test framework is properly set up and working
 */

const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');

// ANSI color codes for better output
const colors = {
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  white: '\x1b[37m',
  reset: '\x1b[0m',
  bold: '\x1b[1m'
};

function log(message, color = 'white') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function logHeader(message) {
  console.log(`\n${colors.bold}${colors.cyan}${message}${colors.reset}`);
  console.log('='.repeat(message.length));
}

function logSuccess(message) {
  log(`✅ ${message}`, 'green');
}

function logError(message) {
  log(`❌ ${message}`, 'red');
}

function logWarning(message) {
  log(`⚠️  ${message}`, 'yellow');
}

function logInfo(message) {
  log(`ℹ️  ${message}`, 'blue');
}

// Validation functions
async function validateFileStructure() {
  logHeader('Validating Test Framework File Structure');
  
  const requiredFiles = [
    'lib/__tests__/runtime-detector.test.js',
    'lib/__tests__/runtime-detector.jest.test.js',
    'lib/__tests__/runtime-detector.integration.test.js',
    'lib/__tests__/setup.js',
    'lib/__tests__/README.md',
    'lib/runtime-detector.js',
    'lib/check-runtimes.js',
    'jest.config.js',
    'package.json'
  ];
  
  let allFilesExist = true;
  
  for (const file of requiredFiles) {
    const filePath = path.join(process.cwd(), file);
    if (fs.existsSync(filePath)) {
      logSuccess(`Found: ${file}`);
    } else {
      logError(`Missing: ${file}`);
      allFilesExist = false;
    }
  }
  
  return allFilesExist;
}

async function validatePackageJson() {
  logHeader('Validating package.json Configuration');
  
  try {
    const packagePath = path.join(process.cwd(), 'package.json');
    const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
    
    // Check for required scripts
    const requiredScripts = [
      'test:runtime',
      'test:jest',
      'test:jest:coverage',
      'test:integration',
      'test:all',
      'check-runtimes'
    ];
    
    let allScriptsPresent = true;
    
    for (const script of requiredScripts) {
      if (packageJson.scripts && packageJson.scripts[script]) {
        logSuccess(`Script found: ${script}`);
      } else {
        logError(`Script missing: ${script}`);
        allScriptsPresent = false;
      }
    }
    
    // Check for Jest dependency
    if (packageJson.devDependencies && packageJson.devDependencies.jest) {
      logSuccess(`Jest dependency: ${packageJson.devDependencies.jest}`);
    } else {
      logError('Jest dependency missing in devDependencies');
      allScriptsPresent = false;
    }
    
    return allScriptsPresent;
  } catch (error) {
    logError(`Failed to validate package.json: ${error.message}`);
    return false;
  }
}

async function validateJestConfig() {
  logHeader('Validating Jest Configuration');
  
  try {
    const jestConfigPath = path.join(process.cwd(), 'jest.config.js');
    
    if (!fs.existsSync(jestConfigPath)) {
      logError('jest.config.js not found');
      return false;
    }
    
    // Try to require the config to check for syntax errors
    const jestConfig = require(jestConfigPath);
    
    logSuccess('Jest configuration loaded successfully');
    
    // Check for important configuration options
    const importantOptions = [
      'testEnvironment',
      'testMatch',
      'collectCoverage',
      'coverageThreshold',
      'setupFilesAfterEnv'
    ];
    
    for (const option of importantOptions) {
      if (jestConfig[option]) {
        logSuccess(`Jest config has: ${option}`);
      } else {
        logWarning(`Jest config missing: ${option}`);
      }
    }
    
    return true;
  } catch (error) {
    logError(`Jest configuration validation failed: ${error.message}`);
    return false;
  }
}

async function validateRuntimeDetector() {
  logHeader('Validating RuntimeDetector Module');
  
  try {
    const RuntimeDetector = require('../lib/runtime-detector');
    
    // Check for required methods
    const requiredMethods = [
      'detectGo',
      'detectPython',
      'detectNodejs',
      'detectJava',
      'validateVersions',
      'generateReport'
    ];
    
    const requiredPrivateMethods = [
      '_executeCommand',
      '_parseGoVersion',
      '_compareVersions',
      '_checkGoVersionCompatibility'
    ];
    
    let allMethodsPresent = true;
    
    for (const method of requiredMethods) {
      if (typeof RuntimeDetector[method] === 'function') {
        logSuccess(`Public method: ${method}`);
      } else {
        logError(`Missing public method: ${method}`);
        allMethodsPresent = false;
      }
    }
    
    for (const method of requiredPrivateMethods) {
      if (typeof RuntimeDetector[method] === 'function') {
        logSuccess(`Private method: ${method}`);
      } else {
        logError(`Missing private method: ${method}`);
        allMethodsPresent = false;
      }
    }
    
    // Test basic functionality
    try {
      const goResult = await RuntimeDetector.detectGo();
      if (goResult && typeof goResult === 'object' && goResult.runtime === 'go') {
        logSuccess('Go detection method works');
      } else {
        logError('Go detection method returned invalid result');
        allMethodsPresent = false;
      }
    } catch (error) {
      logError(`Go detection method failed: ${error.message}`);
      allMethodsPresent = false;
    }
    
    return allMethodsPresent;
  } catch (error) {
    logError(`RuntimeDetector module validation failed: ${error.message}`);
    return false;
  }
}

function runCommand(command, options = {}) {
  return new Promise((resolve, reject) => {
    exec(command, options, (error, stdout, stderr) => {
      resolve({
        success: !error,
        stdout,
        stderr,
        error
      });
    });
  });
}

async function validateTestExecution() {
  logHeader('Validating Test Execution');
  
  const tests = [
    {
      name: 'Custom Test Runner',
      command: 'timeout 30 node lib/__tests__/runtime-detector.test.js',
      timeout: 35000
    },
    {
      name: 'Jest Tests (quick)',
      command: 'timeout 30 npx jest --testPathPattern=jest.test.js --testTimeout=10000',
      timeout: 35000
    },
    {
      name: 'Runtime Check Script',
      command: 'timeout 10 node lib/check-runtimes.js',
      timeout: 15000
    }
  ];
  
  let allTestsPassed = true;
  
  for (const test of tests) {
    logInfo(`Running: ${test.name}`);
    
    try {
      const result = await runCommand(test.command, { 
        timeout: test.timeout,
        env: { ...process.env, SKIP_REAL_COMMANDS: 'true' }
      });
      
      if (result.success) {
        logSuccess(`${test.name} executed successfully`);
      } else {
        logError(`${test.name} failed: ${result.error ? result.error.message : 'Unknown error'}`);
        if (result.stderr) {
          console.log(`  stderr: ${result.stderr.slice(0, 200)}...`);
        }
        allTestsPassed = false;
      }
    } catch (error) {
      logError(`${test.name} execution failed: ${error.message}`);
      allTestsPassed = false;
    }
  }
  
  return allTestsPassed;
}

async function validateTestFramework() {
  logHeader('Test Framework Validation Results');
  
  try {
    const results = {
      fileStructure: await validateFileStructure(),
      packageJson: await validatePackageJson(),
      jestConfig: await validateJestConfig(),
      runtimeDetector: await validateRuntimeDetector(),
      testExecution: await validateTestExecution()
    };
    
    logHeader('Summary');
    
    let allValid = true;
    for (const [category, isValid] of Object.entries(results)) {
      if (isValid) {
        logSuccess(`${category}: Valid`);
      } else {
        logError(`${category}: Invalid`);
        allValid = false;
      }
    }
    
    if (allValid) {
      logSuccess('\n🎉 Test framework validation completed successfully!');
      logInfo('You can now run tests using:');
      logInfo('  npm run test:all          # Run all tests');
      logInfo('  npm run test:runtime      # Custom test runner');
      logInfo('  npm run test:jest         # Jest unit tests');
      logInfo('  npm run test:integration  # Integration tests');
      logInfo('  npm run check-runtimes    # Runtime detection check');
    } else {
      logError('\n❌ Test framework validation failed!');
      logInfo('Please fix the issues above and run validation again.');
    }
    
    return allValid;
  } catch (error) {
    logError(`Validation failed with error: ${error.message}`);
    return false;
  }
}

// Main execution
if (require.main === module) {
  console.log(`${colors.bold}${colors.magenta}LSP Gateway Test Framework Validation${colors.reset}`);
  console.log(`${colors.cyan}Checking test framework setup and functionality...${colors.reset}\n`);
  
  validateTestFramework().then(success => {
    process.exit(success ? 0 : 1);
  }).catch(error => {
    logError(`Fatal validation error: ${error.message}`);
    process.exit(1);
  });
}

module.exports = {
  validateFileStructure,
  validatePackageJson,
  validateJestConfig,
  validateRuntimeDetector,
  validateTestExecution,
  validateTestFramework
};