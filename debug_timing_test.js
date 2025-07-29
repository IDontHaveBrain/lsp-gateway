const { spawn } = require('child_process');

console.log('Testing LSP timing and workspace indexing...');

const gopls = spawn('gopls', [], {
    stdio: ['pipe', 'pipe', 'pipe']
});

const initRequest = {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: {
        processId: process.pid,
        rootPath: process.cwd(),
        rootUri: `file://${process.cwd()}`,
        capabilities: {},
        workspaceFolders: [{
            uri: `file://${process.cwd()}`,
            name: "lsp-gateway"
        }]
    }
};

let buffer = '';
let initialized = false;
let packagesFinishedLoading = false;

gopls.stdout.on('data', (data) => {
    buffer += data.toString();
    
    // Process complete messages
    while (true) {
        const headerMatch = buffer.match(/Content-Length: (\d+)\r\n\r\n/);
        if (!headerMatch) break;
        
        const contentLength = parseInt(headerMatch[1]);
        const headerEnd = headerMatch.index + headerMatch[0].length;
        
        if (buffer.length < headerEnd + contentLength) break;
        
        const messageContent = buffer.substring(headerEnd, headerEnd + contentLength);
        buffer = buffer.substring(headerEnd + contentLength);
        
        try {
            const message = JSON.parse(messageContent);
            console.log(`[${new Date().toISOString()}] Received:`, message.method || `response-${message.id}`, 
                       message.params?.message || 'no-message');
            
            if (message.method === 'window/showMessage' && 
                message.params?.message === 'Finished loading packages.') {
                packagesFinishedLoading = true;
                console.log('📦 Packages finished loading - now sending workspace/symbol request');
                
                setTimeout(() => {
                    sendWorkspaceSymbolRequest();
                }, 100); // Small delay to ensure indexing is complete
            }
        } catch (e) {
            console.log('Failed to parse message:', messageContent);
        }
    }
});

gopls.stderr.on('data', (data) => {
    console.log('Error:', data.toString());
});

function sendWorkspaceSymbolRequest() {
    const symbolRequest = JSON.stringify({
        jsonrpc: "2.0",
        id: 2,
        method: "workspace/symbol",
        params: { query: "main" }
    }) + '\r\n';
    console.log('🔍 Sending workspace/symbol request after packages loaded...');
    gopls.stdin.write(`Content-Length: ${symbolRequest.length}\r\n\r\n${symbolRequest}`);
}

// Send initialize request
const message = JSON.stringify(initRequest) + '\r\n';
console.log('🚀 Sending initialize request...');
gopls.stdin.write(`Content-Length: ${message.length}\r\n\r\n${message}`);

setTimeout(() => {
    if (initialized) {
        // Send initialized notification
        const initialized_msg = JSON.stringify({
            jsonrpc: "2.0",
            method: "initialized",
            params: {}
        }) + '\r\n';
        console.log('✅ Sending initialized notification...');
        gopls.stdin.write(`Content-Length: ${initialized_msg.length}\r\n\r\n${initialized_msg}`);
    }
}, 2000);

setTimeout(() => {
    if (!packagesFinishedLoading) {
        console.log('⚠️ Packages still loading after 10s, sending request anyway...');
        sendWorkspaceSymbolRequest();
    }
}, 10000);

setTimeout(() => {
    gopls.kill();
    console.log('🏁 Test completed');
}, 15000);

gopls.on('close', (code) => {
    console.log(`gopls exited with code ${code}`);
});

// Mark as initialized when we get the initialize response
setTimeout(() => { initialized = true; }, 1000);