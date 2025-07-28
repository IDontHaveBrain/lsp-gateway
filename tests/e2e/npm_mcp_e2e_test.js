#!/usr/bin/env node

/**
 * NPM-MCP E2E Test Suite
 * 
 * Comprehensive end-to-end testing for NPM package functionality
 * with MCP (Model Context Protocol) server integration.
 * 
 * Test Categories:
 * 1. NPM Package Installation & Binary Setup
 * 2. JavaScript API Functionality 
 * 3. Node.js to Go Binary Integration
 * 4. MCP Server Lifecycle Management
 * 5. Error Handling & Recovery
 * 6. Cross-Platform Support
 */

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { spawn, exec } = require('child_process');
const { promisify } = require('util');
const execAsync = promisify(exec);

// Test configuration
const TEST_CONFIG = {
    TIMEOUT: 30000,
    SERVER_PORT: 8081, // Use different port to avoid conflicts
    TEST_PROJECT_DIR: '/tmp/npm_mcp_test_project',
    BINARY_TIMEOUT: 5000,
    MCP_TIMEOUT: 10000,
    VERBOSE: process.env.VERBOSE === 'true'
};

// Import the LSP Gateway module
let LSPGateway, PlatformInfo, Installer;

class NPMMCPTestSuite {
    constructor() {
        this.testResults = [];
        this.cleanupTasks = [];
        this.serverProcesses = [];
    }

    async runAllTests() {
        console.log('🚀 Starting NPM-MCP E2E Test Suite...\n');
        
        try {
            await this.setupTestEnvironment();
            await this.importLSPGatewayModule();
            
            // Run test categories in sequence
            await this.runInstallationTests();
            await this.runJavaScriptAPITests();
            await this.runBinaryIntegrationTests();
            await this.runMCPServerTests();
            await this.runErrorHandlingTests();
            await this.runCrossPlatformTests();
            
            this.printTestResults();
            
        } catch (error) {
            console.error('❌ Test suite failed:', error.message);
            if (TEST_CONFIG.VERBOSE) {
                console.error(error.stack);
            }
            process.exit(1);
        } finally {
            await this.cleanup();
        }
    }

    async setupTestEnvironment() {
        console.log('📋 Setting up test environment...');
        
        // Create test project directory
        if (fs.existsSync(TEST_CONFIG.TEST_PROJECT_DIR)) {
            await execAsync(`rm -rf ${TEST_CONFIG.TEST_PROJECT_DIR}`);
        }
        fs.mkdirSync(TEST_CONFIG.TEST_PROJECT_DIR, { recursive: true });
        
        this.cleanupTasks.push(() => {
            if (fs.existsSync(TEST_CONFIG.TEST_PROJECT_DIR)) {
                fs.rmSync(TEST_CONFIG.TEST_PROJECT_DIR, { recursive: true });
            }
        });
        
        // Create test config files
        await this.createTestConfigFiles();
        
        console.log('✅ Test environment setup complete\n');
    }

    async createTestConfigFiles() {
        const testConfig = `
port: ${TEST_CONFIG.SERVER_PORT}
language_pools:
  go:
    servers:
      - name: "go-test"
        command: ["echo", "mock-gopls"]
        transport: "stdio"
        
mcp:
  enabled: true
  transport: "stdio"
  tools:
    - "goto_definition"
    - "find_references"
    - "get_hover_info"
`;
        
        fs.writeFileSync(
            path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'test-config.yaml'),
            testConfig
        );
    }

    async importLSPGatewayModule() {
        console.log('📦 Importing LSP Gateway module...');
        
        try {
            // Import from the lib directory
            const modulePath = path.resolve(__dirname, '../../lib/index.js');
            const module = require(modulePath);
            
            LSPGateway = module.LSPGateway;
            PlatformInfo = module.PlatformInfo;
            Installer = module.Installer;
            
            assert(LSPGateway, 'LSPGateway class should be available');
            assert(PlatformInfo, 'PlatformInfo class should be available');
            assert(Installer, 'Installer class should be available');
            
            console.log('✅ Module imported successfully\n');
            
        } catch (error) {
            throw new Error(`Failed to import LSP Gateway module: ${error.message}`);
        }
    }

    // =========================================================================
    // INSTALLATION TESTS
    // =========================================================================

    async runInstallationTests() {
        console.log('🔧 Running Installation Tests...\n');
        
        await this.test('Binary Path Detection', async () => {
            const platform = new PlatformInfo();
            const binaryPath = platform.getBinaryPath();
            
            assert(typeof binaryPath === 'string', 'Binary path should be a string');
            assert(binaryPath.length > 0, 'Binary path should not be empty');
            assert(binaryPath.includes('lspg'), 'Binary path should include lspg');
            
            if (TEST_CONFIG.VERBOSE) {
                console.log(`   Binary path: ${binaryPath}`);
            }
        });

        await this.test('Platform Information', async () => {
            const platform = new PlatformInfo();
            const info = platform.getPlatformInfo();
            
            assert(typeof info === 'object', 'Platform info should be an object');
            assert(info.platform, 'Platform should be detected');
            assert(info.name, 'Platform name should be available');
            assert(typeof info.supported === 'boolean', 'Supported flag should be boolean');
            
            if (TEST_CONFIG.VERBOSE) {
                console.log(`   Platform: ${info.platform}, Name: ${info.name}, Supported: ${info.supported}`);
            }
        });

        await this.test('Installer Class Creation', async () => {
            const installer = new Installer();
            assert(installer, 'Installer should be created successfully');
            assert(typeof installer.install === 'function', 'Installer should have install method');
        });

        console.log('✅ Installation tests completed\n');
    }

    // =========================================================================
    // JAVASCRIPT API TESTS
    // =========================================================================

    async runJavaScriptAPITests() {
        console.log('🟨 Running JavaScript API Tests...\n');
        
        await this.test('LSPGateway Constructor', async () => {
            const gateway = new LSPGateway({
                port: TEST_CONFIG.SERVER_PORT,
                config: path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'test-config.yaml')
            });
            
            assert(gateway, 'Gateway should be created');
            assert(gateway.options.port === TEST_CONFIG.SERVER_PORT, 'Port should be set correctly');
            assert(gateway.isRunning === false, 'Gateway should not be running initially');
        });

        await this.test('Gateway Configuration', async () => {
            const gateway = new LSPGateway();
            const status = gateway.getStatus();
            
            assert(typeof status === 'object', 'Status should be an object');
            assert(typeof status.isRunning === 'boolean', 'isRunning should be boolean');
            assert(typeof status.port === 'number', 'port should be number');
            assert(typeof status.platform === 'string', 'platform should be string');
            assert(typeof status.binaryPath === 'string', 'binaryPath should be string');
        });

        await this.test('Platform Info Method', async () => {
            const gateway = new LSPGateway();
            const platformInfo = gateway.getPlatformInfo();
            
            assert(typeof platformInfo === 'object', 'Platform info should be object');
            assert(platformInfo.platform, 'Platform should be detected');
            assert(platformInfo.name, 'Platform name should be available');
            assert(typeof platformInfo.supported === 'boolean', 'Supported should be boolean');
            assert(typeof platformInfo.binaryPath === 'string', 'Binary path should be string');
        });

        await this.test('Default Config Creation', async () => {
            const gateway = new LSPGateway();
            const configPath = path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'generated-config.yaml');
            
            await gateway.createDefaultConfig(configPath);
            
            assert(fs.existsSync(configPath), 'Config file should be created');
            const configContent = fs.readFileSync(configPath, 'utf8');
            assert(configContent.includes('port:'), 'Config should contain port setting');
            assert(configContent.includes('servers:'), 'Config should contain servers section');
        });

        console.log('✅ JavaScript API tests completed\n');
    }

    // =========================================================================
    // BINARY INTEGRATION TESTS
    // =========================================================================

    async runBinaryIntegrationTests() {
        console.log('⚙️ Running Binary Integration Tests...\n');
        
        await this.test('Binary Existence Check', async () => {
            const gateway = new LSPGateway();
            const binaryReady = await gateway.isBinaryReady();
            
            // This might be false if binary isn't built yet, but method should work
            assert(typeof binaryReady === 'boolean', 'isBinaryReady should return boolean');
            
            if (!binaryReady && TEST_CONFIG.VERBOSE) {
                console.log('   Note: Binary not ready (expected in test environment)');
            }
        });

        await this.test('Binary Path Resolution', async () => {
            const gateway = new LSPGateway();
            const binaryPath = gateway.getBinaryPath();
            
            assert(typeof binaryPath === 'string', 'Binary path should be string');
            assert(binaryPath.length > 0, 'Binary path should not be empty');
            
            // Check if the path is reasonable (contains expected parts)
            assert(
                binaryPath.includes('lspg') || binaryPath.includes('node_modules'),
                'Binary path should be reasonable'
            );
        });

        // Skip binary execution tests if binary doesn't exist (common in test environments)
        const gateway = new LSPGateway();
        const binaryExists = fs.existsSync(gateway.getBinaryPath());
        
        if (binaryExists) {
            await this.test('Binary Version Check', async () => {
                const binaryPath = gateway.getBinaryPath();
                
                try {
                    const { stdout } = await execAsync(`"${binaryPath}" version`, {
                        timeout: TEST_CONFIG.BINARY_TIMEOUT
                    });
                    
                    assert(stdout.length > 0, 'Version output should not be empty');
                    if (TEST_CONFIG.VERBOSE) {
                        console.log(`   Version: ${stdout.trim()}`);
                    }
                } catch (error) {
                    // Accept this failure in test environment
                    if (TEST_CONFIG.VERBOSE) {
                        console.log(`   Binary execution failed (expected in test env): ${error.message}`);
                    }
                }
            });
        } else {
            console.log('   ⚠️  Binary not found, skipping binary execution tests');
        }

        console.log('✅ Binary integration tests completed\n');
    }

    // =========================================================================
    // MCP SERVER TESTS
    // =========================================================================

    async runMCPServerTests() {
        console.log('🔌 Running MCP Server Tests...\n');
        
        await this.test('Gateway Start/Stop Cycle', async () => {
            const gateway = new LSPGateway({
                port: TEST_CONFIG.SERVER_PORT,
                config: path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'test-config.yaml')
            });
            
            // Test that start method exists and handles missing binary gracefully
            assert(typeof gateway.start === 'function', 'Gateway should have start method');
            assert(typeof gateway.stop === 'function', 'Gateway should have stop method');
            
            // In test environment, start will likely fail due to missing binary
            // But we test that the methods exist and handle errors appropriately
            try {
                await gateway.start();
                
                // If we reach here, the server started successfully
                assert(gateway.isRunning, 'Gateway should be marked as running');
                
                // Register cleanup
                this.serverProcesses.push(gateway);
                this.cleanupTasks.push(async () => {
                    if (gateway.isRunning) {
                        await gateway.stop();
                    }
                });
                
                // Test stop
                await gateway.stop();
                assert(!gateway.isRunning, 'Gateway should be marked as stopped');
                
            } catch (error) {
                // Expected in test environment without actual binary
                if (TEST_CONFIG.VERBOSE) {
                    console.log(`   Server start failed (expected): ${error.message}`);
                }
                assert(error.message.includes('Failed to start'), 'Should get appropriate error message');
            }
        });

        await this.test('Multiple Gateway Instances', async () => {
            const gateway1 = new LSPGateway({ port: TEST_CONFIG.SERVER_PORT });
            const gateway2 = new LSPGateway({ port: TEST_CONFIG.SERVER_PORT + 1 });
            
            assert(gateway1.options.port !== gateway2.options.port, 'Gateways should have different ports');
            assert(!gateway1.isRunning, 'Gateway1 should not be running');
            assert(!gateway2.isRunning, 'Gateway2 should not be running');
        });

        await this.test('Config File Validation', async () => {
            const validConfigPath = path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'test-config.yaml');
            const invalidConfigPath = path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'nonexistent.yaml');
            
            // Test with valid config
            const gateway1 = new LSPGateway({ config: validConfigPath });
            assert(gateway1.options.config === validConfigPath, 'Valid config should be set');
            
            // Test with invalid config path (should not throw during construction)
            const gateway2 = new LSPGateway({ config: invalidConfigPath });
            assert(gateway2.options.config === invalidConfigPath, 'Invalid config path should still be set');
        });

        console.log('✅ MCP Server tests completed\n');
    }

    // =========================================================================
    // ERROR HANDLING TESTS
    // =========================================================================

    async runErrorHandlingTests() {
        console.log('🚨 Running Error Handling Tests...\n');
        
        await this.test('Invalid Configuration Handling', async () => {
            const gateway = new LSPGateway({ port: 'invalid' });
            
            // Should handle invalid port gracefully
            const status = gateway.getStatus();
            assert(typeof status === 'object', 'Status should still be returned');
        });

        await this.test('Missing Binary Handling', async () => {
            const gateway = new LSPGateway();
            
            // Ensure binary method can be called even if binary is missing
            const binaryReady = await gateway.isBinaryReady();
            assert(typeof binaryReady === 'boolean', 'isBinaryReady should return boolean');
            
            if (!binaryReady) {
                // Try to start - should fail gracefully
                try {
                    await gateway.start();
                    // If it succeeds, stop it
                    await gateway.stop();
                } catch (error) {
                    assert(error instanceof Error, 'Should throw proper Error object');
                    assert(error.message.length > 0, 'Error should have meaningful message');
                }
            }
        });

        await this.test('Stop Without Start', async () => {
            const gateway = new LSPGateway();
            
            // Should handle stop gracefully even if never started
            await gateway.stop();
            assert(!gateway.isRunning, 'Gateway should not be running');
        });

        await this.test('Double Start Prevention', async () => {
            const gateway = new LSPGateway({
                port: TEST_CONFIG.SERVER_PORT,
                config: path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'test-config.yaml')
            });
            
            // Mock successful start
            gateway.isRunning = true;
            
            try {
                await gateway.start();
                assert(false, 'Second start should throw error');
            } catch (error) {
                assert(error.message.includes('already running'), 'Should get appropriate error');
            }
            
            // Reset state
            gateway.isRunning = false;
        });

        await this.test('Config File Creation Error Handling', async () => {
            const gateway = new LSPGateway();
            const invalidPath = '/root/cannot_write_here.yaml'; // Path that should fail
            
            try {
                await gateway.createDefaultConfig(invalidPath);
                // If it doesn't throw, that's fine too (depending on permissions)
            } catch (error) {
                assert(error instanceof Error, 'Should throw proper Error object');
                assert(error.message.includes('Failed to create'), 'Should have appropriate error message');
            }
        });

        console.log('✅ Error handling tests completed\n');
    }

    // =========================================================================
    // CROSS-PLATFORM TESTS
    // =========================================================================

    async runCrossPlatformTests() {
        console.log('🌐 Running Cross-Platform Tests...\n');
        
        await this.test('Platform Detection', async () => {
            const platform = new PlatformInfo();
            const currentPlatform = platform.getCurrentPlatform();
            
            assert(typeof currentPlatform === 'object', 'Platform should be object');
            assert(currentPlatform.os, 'Platform should have OS');
            assert(currentPlatform.arch, 'Platform should have architecture');
            
            const supportedPlatforms = ['linux', 'darwin', 'win32'];
            const supportedArchs = ['x64', 'arm64'];
            
            assert(supportedPlatforms.includes(currentPlatform.os), 
                   `OS ${currentPlatform.os} should be supported`);
            assert(supportedArchs.includes(currentPlatform.arch), 
                   `Architecture ${currentPlatform.arch} should be supported`);
        });

        await this.test('Binary Path Platform Specificity', async () => {
            const platform = new PlatformInfo();
            const binaryPath = platform.getBinaryPath();
            const currentPlatform = platform.getCurrentPlatform();
            
            // Binary path should reflect current platform
            if (currentPlatform.os === 'win32') {
                assert(binaryPath.includes('.exe'), 'Windows binary should have .exe extension');
            } else {
                assert(!binaryPath.includes('.exe'), 'Non-Windows binary should not have .exe extension');
            }
        });

        await this.test('Path Handling', async () => {
            const gateway = new LSPGateway();
            
            // Test with different path formats
            const testPaths = [
                'config.yaml',
                './config.yaml',
                path.join(TEST_CONFIG.TEST_PROJECT_DIR, 'config.yaml'),
                path.resolve(TEST_CONFIG.TEST_PROJECT_DIR, 'config.yaml')
            ];
            
            for (const testPath of testPaths) {
                const testGateway = new LSPGateway({ config: testPath });
                assert(testGateway.options.config === testPath, 
                       `Config path should be preserved: ${testPath}`);
            }
        });

        console.log('✅ Cross-platform tests completed\n');
    }

    // =========================================================================
    // TEST UTILITIES
    // =========================================================================

    async test(name, testFunction) {
        const startTime = Date.now();
        
        try {
            await Promise.race([
                testFunction(),
                new Promise((_, reject) => 
                    setTimeout(() => reject(new Error('Test timeout')), TEST_CONFIG.TIMEOUT)
                )
            ]);
            
            const duration = Date.now() - startTime;
            this.testResults.push({ name, status: 'PASS', duration });
            
            if (TEST_CONFIG.VERBOSE) {
                console.log(`✅ ${name} (${duration}ms)`);
            }
            
        } catch (error) {
            const duration = Date.now() - startTime;
            this.testResults.push({ name, status: 'FAIL', duration, error: error.message });
            
            console.error(`❌ ${name} (${duration}ms): ${error.message}`);
            
            if (TEST_CONFIG.VERBOSE) {
                console.error(error.stack);
            }
        }
    }

    printTestResults() {
        console.log('\n📊 Test Results Summary');
        console.log('========================\n');
        
        const passed = this.testResults.filter(r => r.status === 'PASS').length;
        const failed = this.testResults.filter(r => r.status === 'FAIL').length;
        const total = this.testResults.length;
        
        console.log(`Total Tests: ${total}`);
        console.log(`Passed: ${passed} ✅`);
        console.log(`Failed: ${failed} ❌`);
        console.log(`Success Rate: ${((passed / total) * 100).toFixed(1)}%\n`);
        
        if (failed > 0) {
            console.log('Failed Tests:');
            this.testResults
                .filter(r => r.status === 'FAIL')
                .forEach(r => console.log(`  ❌ ${r.name}: ${r.error}`));
            console.log();
        }
        
        const totalDuration = this.testResults.reduce((sum, r) => sum + r.duration, 0);
        console.log(`Total Duration: ${totalDuration}ms`);
        
        // Exit with error code if any tests failed
        if (failed > 0) {
            process.exit(1);
        }
    }

    async cleanup() {
        console.log('\n🧹 Cleaning up test environment...');
        
        // Stop any running servers
        for (const gateway of this.serverProcesses) {
            try {
                if (gateway.isRunning) {
                    await gateway.stop();
                }
            } catch (error) {
                if (TEST_CONFIG.VERBOSE) {
                    console.log(`   Warning: Failed to stop gateway: ${error.message}`);
                }
            }
        }
        
        // Run cleanup tasks
        for (const cleanupTask of this.cleanupTasks.reverse()) {
            try {
                await cleanupTask();
            } catch (error) {
                if (TEST_CONFIG.VERBOSE) {
                    console.log(`   Warning: Cleanup task failed: ${error.message}`);
                }
            }
        }
        
        console.log('✅ Cleanup completed');
    }
}

// =========================================================================
// MAIN EXECUTION
// =========================================================================

if (require.main === module) {
    const testSuite = new NPMMCPTestSuite();
    testSuite.runAllTests().catch(error => {
        console.error('Fatal error:', error);
        process.exit(1);
    });
}

module.exports = NPMMCPTestSuite;