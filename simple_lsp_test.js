const { spawn } = require('child_process');

console.log('Testing direct LSP communication...');

const gopls = spawn('gopls', [], {
    stdio: ['pipe', 'pipe', 'pipe']
});

// LSP initialize request
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
gopls.stdout.on('data', (data) => {
    buffer += data.toString();
    console.log('Received:', data.toString());
});

gopls.stderr.on('data', (data) => {
    console.log('Error:', data.toString());
});

// Send initialize request
const message = JSON.stringify(initRequest) + '\r\n';
console.log('Sending initialize request...');
gopls.stdin.write(`Content-Length: ${message.length}\r\n\r\n${message}`);

setTimeout(() => {
    // Send initialized notification
    const initialized = JSON.stringify({
        jsonrpc: "2.0",
        method: "initialized",
        params: {}
    }) + '\r\n';
    console.log('Sending initialized notification...');
    gopls.stdin.write(`Content-Length: ${initialized.length}\r\n\r\n${initialized}`);
    
    setTimeout(() => {
        // Test workspace/symbol
        const symbolRequest = JSON.stringify({
            jsonrpc: "2.0",
            id: 2,
            method: "workspace/symbol",
            params: { query: "main" }
        }) + '\r\n';
        console.log('Sending workspace/symbol request...');
        gopls.stdin.write(`Content-Length: ${symbolRequest.length}\r\n\r\n${symbolRequest}`);
        
        setTimeout(() => {
            gopls.kill();
            console.log('Test completed');
        }, 3000);
    }, 1000);
}, 2000);

gopls.on('exit', (code) => {
    console.log(`gopls exited with code ${code}`);
});