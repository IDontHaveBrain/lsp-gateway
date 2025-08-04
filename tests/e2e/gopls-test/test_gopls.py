#!/usr/bin/env python3
import subprocess
import json
import sys
import os

def send_lsp_message(proc, method, params=None, request_id=1):
    message = {
        "jsonrpc": "2.0",
        "id": request_id,
        "method": method,
        "params": params
    }
    
    content = json.dumps(message)
    header = f"Content-Length: {len(content)}\r\n\r\n"
    full_message = header + content
    
    print(f"Sending: {method}")
    print(f"Message: {content}")
    
    proc.stdin.write(full_message.encode())
    proc.stdin.flush()
    
    # Read response
    response_header = b""
    while b"\r\n\r\n" not in response_header:
        char = proc.stdout.read(1)
        if not char:
            print("EOF while reading header")
            return None
        response_header += char
    
    # Parse content length
    header_str = response_header.decode()
    content_length = 0
    for line in header_str.split('\r\n'):
        if line.startswith('Content-Length:'):
            content_length = int(line.split(':')[1].strip())
            break
    
    if content_length > 0:
        content = proc.stdout.read(content_length).decode()
        print(f"Received: {content}")
        return json.loads(content)
    
    return None

def main():
    os.chdir(os.path.expanduser("~/work/lsp-gateway/tests/e2e/gopls-test"))
    
    # Start gopls
    proc = subprocess.Popen(
        ["gopls", "serve"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=os.path.expanduser("~/work/lsp-gateway/tests/e2e/gopls-test")
    )
    
    try:
        # Initialize
        init_params = {
            "processId": os.getpid(),
            "rootUri": f"file://{os.path.expanduser('~/work/lsp-gateway/tests/e2e/gopls-test')}",
            "capabilities": {
                "textDocument": {
                    "hover": {"contentFormat": ["markdown", "plaintext"]},
                    "definition": {"dynamicRegistration": True}
                }
            }
        }
        
        response = send_lsp_message(proc, "initialize", init_params, 1)
        if response:
            print("Initialize OK")
            
            # Send initialized notification
            message = {
                "jsonrpc": "2.0",
                "method": "initialized",
                "params": {}
            }
            content = json.dumps(message)
            header = f"Content-Length: {len(content)}\r\n\r\n"
            proc.stdin.write((header + content).encode())
            proc.stdin.flush()
            print("Sent initialized notification")
            
            # Test didOpen
            didopen_params = {
                "textDocument": {
                    "uri": f"file://{os.path.expanduser('~/work/lsp-gateway/tests/e2e/gopls-test/test.go')}",
                    "languageId": "go",
                    "version": 1,
                    "text": """package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

func main() {
	p := Person{Name: "John", Age: 30}
	fmt.Println(p.Name)
}"""
                }
            }
            
            # Send didOpen (notification, no response expected)
            message = {
                "jsonrpc": "2.0",
                "method": "textDocument/didOpen",
                "params": didopen_params
            }
            content = json.dumps(message)
            header = f"Content-Length: {len(content)}\r\n\r\n"
            proc.stdin.write((header + content).encode())
            proc.stdin.flush()
            print("Sent didOpen notification")
            
            # Wait a bit for processing
            import time
            time.sleep(1)
            
            # Test hover
            hover_params = {
                "textDocument": {
                    "uri": f"file://{os.path.expanduser('~/work/lsp-gateway/tests/e2e/gopls-test/test.go')}"
                },
                "position": {
                    "line": 8,
                    "character": 5
                }
            }
            
            print("Testing hover...")
            response = send_lsp_message(proc, "textDocument/hover", hover_params, 3)
            if response:
                print("Hover worked!")
            else:
                print("Hover failed!")
        
    except Exception as e:
        print(f"Error: {e}")
    finally:
        proc.terminate()
        proc.wait()

if __name__ == "__main__":
    main()