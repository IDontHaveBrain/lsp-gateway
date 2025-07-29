#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');

class MCPClient {
    constructor() {
        this.process = null;
        this.messageId = 1;
        this.pendingRequests = new Map();
    }

    async start() {
        return new Promise((resolve, reject) => {
            console.log('🚀 Starting MCP client test...');
            
            // Start the MCP server process
            this.process = spawn('lspg', ['mcp', '--config', 'config.yaml'], {
                stdio: ['pipe', 'pipe', 'pipe'],
                cwd: process.cwd()
            });

            this.process.stderr.on('data', (data) => {
                console.log(`[MCP-STDERR] ${data.toString().trim()}`);
            });

            this.process.stdout.on('data', (data) => {
                this.handleResponse(data.toString());
            });

            this.process.on('error', (error) => {
                console.error('❌ Process error:', error);
                reject(error);
            });

            // Wait for server to start
            setTimeout(() => {
                console.log('✅ MCP server should be started');
                resolve();
            }, 2000);
        });
    }

    handleResponse(data) {
        const lines = data.split('\n').filter(line => line.trim());
        
        for (const line of lines) {
            try {
                const message = JSON.parse(line);
                console.log(`📨 Received: ${JSON.stringify(message, null, 2)}`);
                
                if (message.id && this.pendingRequests.has(message.id)) {
                    const { resolve, reject } = this.pendingRequests.get(message.id);
                    this.pendingRequests.delete(message.id);
                    
                    if (message.error) {
                        reject(new Error(message.error.message));
                    } else {
                        resolve(message.result);
                    }
                }
            } catch (e) {
                // Ignore non-JSON lines (debug output)
            }
        }
    }

    async sendRequest(method, params = {}, timeoutMs = 30000) {
        return new Promise((resolve, reject) => {
            const id = this.messageId++;
            const message = {
                jsonrpc: "2.0",
                id: id,
                method: method,
                params: params
            };

            console.log(`📤 Sending: ${method} (ID: ${id})`);
            
            // Set up timeout
            const timeout = setTimeout(() => {
                this.pendingRequests.delete(id);
                reject(new Error(`Request timeout after ${timeoutMs}ms: ${method}`));
            }, timeoutMs);

            // Store request
            this.pendingRequests.set(id, {
                resolve: (result) => {
                    clearTimeout(timeout);
                    resolve(result);
                },
                reject: (error) => {
                    clearTimeout(timeout);
                    reject(error);
                }
            });

            // Send message
            const messageStr = JSON.stringify(message) + '\n';
            this.process.stdin.write(messageStr);
        });
    }

    async stop() {
        if (this.process) {
            this.process.kill('SIGTERM');
            this.process = null;
        }
    }
}

async function testMCPTools() {
    const client = new MCPClient();
    
    try {
        console.log('=== MCP Timeout Test Started ===');
        
        // Start MCP server
        await client.start();
        
        // Test 1: Initialize MCP
        console.log('\n🔧 Test 1: MCP Initialize');
        try {
            const initResult = await client.sendRequest('initialize', {
                protocolVersion: '2024-11-05',
                capabilities: {
                    roots: { listChanged: true },
                    sampling: {}
                },
                clientInfo: {
                    name: 'test-client',
                    version: '1.0.0'
                }
            }, 10000);
            console.log('✅ Initialize successful');
        } catch (error) {
            console.log('❌ Initialize failed:', error.message);
        }

        // Test 2: List tools  
        console.log('\n🔧 Test 2: List Tools');
        try {
            const toolsResult = await client.sendRequest('tools/list', {}, 10000);
            console.log('✅ List tools successful:', toolsResult?.tools?.length || 0, 'tools found');
        } catch (error) {
            console.log('❌ List tools failed:', error.message);
        }

        // Test 3: Test a simple LSP tool (get_document_symbols on a Go file)
        console.log('\n🔧 Test 3: Document Symbols (Go file)');
        try {
            const symbolsResult = await client.sendRequest('tools/call', {
                name: 'lspg_get_document_symbols',
                arguments: {
                    uri: `file://${path.resolve('./go.mod')}`
                }
            }, 15000);
            console.log('✅ Document symbols successful');
        } catch (error) {
            console.log('❌ Document symbols failed:', error.message);
        }

        // Test 4: Test workspace symbols
        console.log('\n🔧 Test 4: Workspace Symbols');
        try {
            const workspaceResult = await client.sendRequest('tools/call', {
                name: 'lspg_search_workspace_symbols',
                arguments: {
                    query: 'main'
                }
            }, 15000);
            console.log('✅ Workspace symbols successful');
        } catch (error) {
            console.log('❌ Workspace symbols failed:', error.message);
        }

        // Test 5: Test with potentially problematic timeout scenario
        console.log('\n🔧 Test 5: Timeout Behavior Test');
        try {
            const timeoutResult = await client.sendRequest('tools/call', {
                name: 'lspg_get_document_symbols',
                arguments: {
                    uri: 'file:///nonexistent/file.go'
                }
            }, 5000); // Short timeout to test behavior
            console.log('✅ Timeout test completed');
        } catch (error) {
            console.log('✅ Timeout test behaved correctly:', error.message);
        }

        console.log('\n=== All Tests Completed ===');
        
    } catch (error) {
        console.error('❌ Test failed:', error);
    } finally {
        await client.stop();
        console.log('🔄 MCP client stopped');
    }
}

// Run the test
testMCPTools().catch(console.error);