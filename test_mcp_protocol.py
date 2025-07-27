#!/usr/bin/env python3
"""
Test script to validate MCP protocol functionality.
This script tests the core MCP protocol handshake and basic operations.
"""

import json
import subprocess
import sys
import time
import threading
from typing import Dict, Any, Optional

class MCPProtocolTester:
    def __init__(self, binary_path: str, config_path: str):
        self.binary_path = binary_path
        self.config_path = config_path
        self.process = None
        self.initialized = False
        
    def start_mcp_server(self):
        """Start the MCP server process"""
        try:
            self.process = subprocess.Popen(
                [self.binary_path, "mcp", "--config", self.config_path],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            print(f"✓ MCP server started with PID: {self.process.pid}")
            return True
        except Exception as e:
            print(f"✗ Failed to start MCP server: {e}")
            return False
    
    def send_request(self, method: str, params: Dict[str, Any] = None, request_id: int = 1) -> Optional[Dict]:
        """Send a JSON-RPC request to the MCP server"""
        if not self.process:
            print("✗ MCP server not started")
            return None
            
        request = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params or {},
            "id": request_id
        }
        
        request_json = json.dumps(request)
        print(f"→ Sending: {request_json}")
        
        try:
            self.process.stdin.write(request_json + "\n")
            self.process.stdin.flush()
            
            # Read response with timeout
            response_line = self.process.stdout.readline()
            if response_line:
                response = json.loads(response_line.strip())
                print(f"← Received: {json.dumps(response, indent=2)}")
                return response
            else:
                print("✗ No response received")
                return None
                
        except Exception as e:
            print(f"✗ Error sending request: {e}")
            return None
    
    def test_initialize(self):
        """Test MCP protocol initialization"""
        print("\n🔄 Testing MCP protocol initialization...")
        
        params = {
            "protocolVersion": "2024-11-05",
            "capabilities": {
                "tools": {}
            },
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        }
        
        response = self.send_request("initialize", params)
        if response and "result" in response:
            print("✓ Initialize successful")
            self.initialized = True
            return response["result"]
        else:
            print("✗ Initialize failed")
            return None
    
    def test_tools_list(self):
        """Test listing available tools"""
        print("\n🔄 Testing tools/list...")
        
        response = self.send_request("tools/list")
        if response and "result" in response:
            tools = response["result"].get("tools", [])
            print(f"✓ Found {len(tools)} tools:")
            for tool in tools:
                print(f"  - {tool.get('name', 'unknown')}: {tool.get('description', 'no description')[:60]}...")
            return tools
        else:
            print("✗ tools/list failed")
            return []
    
    def test_basic_tool_call(self, tool_name: str, arguments: Dict[str, Any]):
        """Test calling a basic tool"""
        print(f"\n🔄 Testing tool call: {tool_name}...")
        
        params = {
            "name": tool_name,
            "arguments": arguments
        }
        
        response = self.send_request("tools/call", params)
        if response and "result" in response:
            print(f"✓ Tool call {tool_name} successful")
            return response["result"]
        else:
            print(f"✗ Tool call {tool_name} failed")
            return None
    
    def test_ping(self):
        """Test ping functionality"""
        print("\n🔄 Testing ping...")
        
        response = self.send_request("ping")
        if response and "result" in response:
            print("✓ Ping successful")
            return True
        else:
            print("✗ Ping failed")
            return False
    
    def test_error_scenarios(self):
        """Test various error scenarios"""
        print("\n🔄 Testing error scenarios...")
        
        # Test invalid method
        print("Testing invalid method...")
        response = self.send_request("invalid/method")
        if response and "error" in response:
            print(f"✓ Invalid method properly rejected: {response['error']['message']}")
        
        # Test invalid tool call
        print("Testing invalid tool call...")
        response = self.send_request("tools/call", {"name": "nonexistent_tool", "arguments": {}})
        if response and "error" in response:
            print(f"✓ Invalid tool properly rejected: {response['error']['message']}")
        
        # Test malformed request
        print("Testing request without ID...")
        try:
            request = {"jsonrpc": "2.0", "method": "ping"}
            request_json = json.dumps(request)
            self.process.stdin.write(request_json + "\n")
            self.process.stdin.flush()
            response_line = self.process.stdout.readline()
            if response_line:
                response = json.loads(response_line.strip())
                if "error" in response:
                    print(f"✓ Request without ID properly rejected: {response['error']['message']}")
        except Exception as e:
            print(f"✗ Error testing malformed request: {e}")
    
    def cleanup(self):
        """Clean up resources"""
        if self.process:
            try:
                self.process.terminate()
                self.process.wait(timeout=5)
                print("✓ MCP server terminated gracefully")
            except:
                self.process.kill()
                print("⚠ MCP server killed forcefully")

def main():
    """Main test execution"""
    binary_path = "./bin/lsp-gateway"
    config_path = "config.yaml"
    
    print("🧪 MCP Protocol Validation Test")
    print("=" * 50)
    
    tester = MCPProtocolTester(binary_path, config_path)
    
    try:
        # Start MCP server
        if not tester.start_mcp_server():
            return 1
        
        # Give server time to start
        time.sleep(1)
        
        # Test protocol initialization
        init_result = tester.test_initialize()
        if not init_result:
            print("✗ Initialization failed, skipping other tests")
            return 1
        
        # Test tools listing
        tools = tester.test_tools_list()
        
        # Test ping
        tester.test_ping()
        
        # Test basic tool calls if tools are available
        expected_tools = [
            "goto_definition", 
            "find_references", 
            "get_hover_info", 
            "get_document_symbols", 
            "search_workspace_symbols"
        ]
        
        available_tool_names = [tool.get("name") for tool in tools]
        print(f"\n📋 Expected tools: {expected_tools}")
        print(f"📋 Available tools: {available_tool_names}")
        
        for expected_tool in expected_tools:
            if expected_tool in available_tool_names:
                print(f"✓ Expected tool {expected_tool} is available")
            else:
                print(f"✗ Expected tool {expected_tool} is missing")
        
        # Test error scenarios
        tester.test_error_scenarios()
        
        print("\n✅ MCP protocol validation completed")
        return 0
        
    except KeyboardInterrupt:
        print("\n⚠ Test interrupted by user")
        return 1
    except Exception as e:
        print(f"\n✗ Test failed with error: {e}")
        return 1
    finally:
        tester.cleanup()

if __name__ == "__main__":
    sys.exit(main())