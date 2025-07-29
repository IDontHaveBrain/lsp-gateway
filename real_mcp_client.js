#!/usr/bin/env node

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

class RealMCPClient {
    constructor() {
        this.process = null;
        this.messageId = 1;
        this.pendingRequests = new Map();
        this.initialized = false;
        this.tools = [];
        this.debugLog = [];
    }

    log(message) {
        const timestamp = new Date().toISOString();
        const logEntry = `[${timestamp}] ${message}`;
        console.log(logEntry);
        this.debugLog.push(logEntry);
    }

    async start() {
        return new Promise((resolve, reject) => {
            this.log('🚀 Starting Real MCP Client...');
            
            // Start the MCP server process
            this.process = spawn('lspg', ['mcp', '--config', 'config.yaml'], {
                stdio: ['pipe', 'pipe', 'pipe'],
                cwd: process.cwd()
            });

            // Handle stderr for server status
            this.process.stderr.on('data', (data) => {
                const output = data.toString();
                this.log(`[SERVER-STDERR] ${output.trim()}`);
                
                if (output.includes('MCP server is running')) {
                    this.log('✅ MCP server started successfully');
                    setTimeout(() => resolve(), 2000); // Give server time to fully initialize
                }
            });

            // Handle stdout for MCP responses
            this.process.stdout.on('data', (data) => {
                this.handleResponse(data.toString());
            });

            this.process.on('error', (error) => {
                this.log(`❌ Process error: ${error.message}`);
                reject(error);
            });

            this.process.on('exit', (code) => {
                this.log(`🔄 MCP server exited with code: ${code}`);
            });

            // Safety timeout
            setTimeout(() => {
                if (!this.initialized) {
                    reject(new Error('Server startup timeout'));
                }
            }, 15000);
        });
    }

    handleResponse(data) {
        const lines = data.split('\n').filter(line => line.trim());
        
        for (const line of lines) {
            try {
                const message = JSON.parse(line);
                this.log(`📨 Received response: ID=${message.id}, method=${message.method || 'response'}`);
                
                if (message.id && this.pendingRequests.has(message.id)) {
                    const { resolve, reject, timeout, method } = this.pendingRequests.get(message.id);
                    this.pendingRequests.delete(message.id);
                    clearTimeout(timeout);
                    
                    if (message.error) {
                        this.log(`❌ Request ${message.id} (${method}) failed: ${message.error.message}`);
                        reject(new Error(message.error.message));
                    } else {
                        this.log(`✅ Request ${message.id} (${method}) successful`);
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

            this.log(`📤 Sending request: ID=${id}, method=${method}, timeout=${timeoutMs}ms`);
            this.log(`📤 Request payload: ${JSON.stringify(message, null, 2)}`);
            
            // Set up timeout
            const timeout = setTimeout(() => {
                this.log(`⏰ Request ${id} (${method}) TIMED OUT after ${timeoutMs}ms`);
                this.pendingRequests.delete(id);
                reject(new Error(`Request timeout after ${timeoutMs}ms: ${method}`));
            }, timeoutMs);

            // Store request with debugging info
            this.pendingRequests.set(id, {
                resolve,
                reject,
                timeout,
                method,
                startTime: Date.now()
            });

            // Send message
            const messageStr = JSON.stringify(message) + '\n';
            this.process.stdin.write(messageStr);
            this.log(`📤 Message sent: ${messageStr.trim()}`);
        });
    }

    async initialize() {
        this.log('🔧 Initializing MCP connection...');
        
        const result = await this.sendRequest('initialize', {
            protocolVersion: '2024-11-05',
            capabilities: {
                roots: { listChanged: true },
                sampling: {}
            },
            clientInfo: {
                name: 'real-mcp-test-client',
                version: '1.0.0'
            }
        }, 10000);

        this.initialized = true;
        this.log('✅ MCP initialization completed');
        return result;
    }

    async listTools() {
        this.log('🔧 Listing available tools...');
        
        const result = await this.sendRequest('tools/list', {}, 10000);
        this.tools = result.tools || [];
        
        this.log(`✅ Found ${this.tools.length} tools:`);
        this.tools.forEach((tool, index) => {
            this.log(`   ${index + 1}. ${tool.name} - ${tool.description}`);
        });
        
        return result;
    }

    async callTool(toolName, args, timeoutMs = 45000) {
        this.log(`🔧 Calling tool: ${toolName} with timeout ${timeoutMs}ms`);
        this.log(`🔧 Tool arguments: ${JSON.stringify(args, null, 2)}`);
        
        const startTime = Date.now();
        
        try {
            const result = await this.sendRequest('tools/call', {
                name: toolName,
                arguments: args
            }, timeoutMs);
            
            const duration = Date.now() - startTime;
            this.log(`✅ Tool call completed in ${duration}ms`);
            return result;
            
        } catch (error) {
            const duration = Date.now() - startTime;
            this.log(`❌ Tool call failed after ${duration}ms: ${error.message}`);
            throw error;
        }
    }

    async stop() {
        this.log('🔄 Stopping MCP client...');
        
        // Cancel all pending requests
        for (const [id, { reject, timeout, method }] of this.pendingRequests) {
            clearTimeout(timeout);
            reject(new Error(`Client shutdown: ${method}`));
        }
        this.pendingRequests.clear();

        if (this.process) {
            this.process.kill('SIGTERM');
            this.process = null;
        }
        
        this.log('🔄 MCP client stopped');
    }

    saveDebugLog() {
        const logFile = `mcp_client_debug_${Date.now()}.log`;
        fs.writeFileSync(logFile, this.debugLog.join('\n'));
        this.log(`💾 Debug log saved to: ${logFile}`);
        return logFile;
    }

    showPendingRequests() {
        this.log(`📊 Pending requests: ${this.pendingRequests.size}`);
        for (const [id, { method, startTime }] of this.pendingRequests) {
            const elapsed = Date.now() - startTime;
            this.log(`   ID=${id}, method=${method}, elapsed=${elapsed}ms`);
        }
    }
}

async function runInfiniteWaitingTest() {
    const client = new RealMCPClient();
    
    try {
        console.log('=== Real MCP Client - Infinite Waiting Reproduction Test ===\n');
        
        // Start MCP server
        await client.start();
        
        // Initialize MCP connection
        await client.initialize();
        
        // List available tools
        await client.listTools();
        
        // Test 1: Simple workspace symbols search (this should reproduce the infinite waiting)
        console.log('\n🧪 Test 1: Workspace Symbols Search - INFINITE WAITING TEST');
        try {
            const result = await client.callTool('search_workspace_symbols', {
                query: 'main'
            }, 20000); // 20 second timeout
            
            console.log('✅ Workspace symbols search completed successfully');
            console.log(`📊 Found ${result.content?.[0]?.data?.length || 0} symbols`);
            
        } catch (error) {
            console.log(`❌ Workspace symbols search failed: ${error.message}`);
            client.showPendingRequests();
        }
        
        // Test 2: Document symbols on a specific file  
        console.log('\n🧪 Test 2: Document Symbols - Specific File Test');
        try {
            const goModPath = path.resolve('./go.mod');
            const result = await client.callTool('get_document_symbols', {
                uri: `file://${goModPath}`
            }, 15000);
            
            console.log('✅ Document symbols completed successfully');
            console.log(`📊 Found ${result.content?.[0]?.data?.length || 0} document symbols`);
            
        } catch (error) {
            console.log(`❌ Document symbols failed: ${error.message}`);
            client.showPendingRequests();
        }
        
        // Test 3: Go to definition (this might also hang)
        console.log('\n🧪 Test 3: Go to Definition Test');
        try {
            const result = await client.callTool('goto_definition', {
                uri: `file://${path.resolve('./go.mod')}`,
                line: 0,
                character: 0
            }, 15000);
            
            console.log('✅ Go to definition completed successfully');
            
        } catch (error) {
            console.log(`❌ Go to definition failed: ${error.message}`);
            client.showPendingRequests();
        }
        
        console.log('\n=== Test Completed ===');
        
    } catch (error) {
        console.error(`❌ Test failed: ${error.message}`);
        client.showPendingRequests();
    } finally {
        const logFile = client.saveDebugLog();
        await client.stop();
        console.log(`\n💾 Full debug log available in: ${logFile}`);
    }
}

// Run the test if this file is executed directly
if (require.main === module) {
    runInfiniteWaitingTest().catch(console.error);
}

module.exports = { RealMCPClient };