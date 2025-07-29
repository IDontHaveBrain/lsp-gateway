package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/internal/workspace"
)

// TransportProtocolTestSuite tests transport layer functionality with multiple clients
type TransportProtocolTestSuite struct {
	suite.Suite
	tempDir       string
	workspaceRoot string
	clients       map[string]transport.LSPClient
	configs       map[string]transport.ClientConfig
	ctx           context.Context
	cancel        context.CancelFunc
	clientMutex   sync.RWMutex
}

func TestTransportProtocolTestSuite(t *testing.T) {
	suite.Run(t, new(TransportProtocolTestSuite))
}

func (suite *TransportProtocolTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "transport-protocol-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.clients = make(map[string]transport.LSPClient)
	suite.configs = make(map[string]transport.ClientConfig)
	
	suite.setupMultiProjectWorkspace()
	suite.setupTransportConfigs()
}

func (suite *TransportProtocolTestSuite) TearDownSuite() {
	suite.clientMutex.Lock()
	defer suite.clientMutex.Unlock()
	
	// Stop all LSP clients
	for name, client := range suite.clients {
		if client != nil && client.IsActive() {
			client.Stop()
			suite.T().Logf("Stopped client: %s", name)
		}
	}
	
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *TransportProtocolTestSuite) SetupTest() {
	// Clean up any existing clients
	suite.clientMutex.Lock()
	for name, client := range suite.clients {
		if client != nil && client.IsActive() {
			client.Stop()
		}
		delete(suite.clients, name)
	}
	suite.clientMutex.Unlock()
}

func (suite *TransportProtocolTestSuite) TearDownTest() {
	suite.clientMutex.Lock()
	defer suite.clientMutex.Unlock()
	
	// Stop all test clients
	for name, client := range suite.clients {
		if client != nil && client.IsActive() {
			client.Stop()
		}
	}
	
	// Clear clients map
	suite.clients = make(map[string]transport.LSPClient)
}

func (suite *TransportProtocolTestSuite) setupMultiProjectWorkspace() {
	// Create Go project
	goProject := filepath.Join(suite.workspaceRoot, "go-transport-test")
	suite.Require().NoError(os.MkdirAll(goProject, 0755))
	
	goMod := `module go-transport-test

go 1.21

require (
	github.com/gorilla/websocket v1.5.1
	golang.org/x/sync v0.5.0
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "go.mod"), []byte(goMod), 0644))
	
	transportGo := `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// Transport represents different transport mechanisms
type Transport interface {
	Send(ctx context.Context, data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	Close() error
	IsConnected() bool
	GetType() string
}

// HTTPTransport implements HTTP-based transport
type HTTPTransport struct {
	client  *http.Client
	baseURL string
	mutex   sync.RWMutex
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

func (t *HTTPTransport) Send(ctx context.Context, data []byte) error {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/message", 
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/message", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	return data, nil
}

func (t *HTTPTransport) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// HTTP client doesn't need explicit closing
	return nil
}

func (t *HTTPTransport) IsConnected() bool {
	return true // HTTP is stateless
}

func (t *HTTPTransport) GetType() string {
	return "http"
}

// WebSocketTransport implements WebSocket-based transport
type WebSocketTransport struct {
	conn    *websocket.Conn
	url     string
	mutex   sync.RWMutex
	closed  bool
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(url string) *WebSocketTransport {
	return &WebSocketTransport{
		url: url,
	}
}

func (t *WebSocketTransport) Connect(ctx context.Context) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	
	conn, _, err := dialer.DialContext(ctx, t.url, nil)
	if err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}
	
	t.conn = conn
	t.closed = false
	return nil
}

func (t *WebSocketTransport) Send(ctx context.Context, data []byte) error {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	if t.conn == nil || t.closed {
		return fmt.Errorf("WebSocket not connected")
	}
	
	// Set write deadline
	if deadline, ok := ctx.Deadline(); ok {
		t.conn.SetWriteDeadline(deadline)
	}
	
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

func (t *WebSocketTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	if t.conn == nil || t.closed {
		return nil, fmt.Errorf("WebSocket not connected")
	}
	
	// Set read deadline
	if deadline, ok := ctx.Deadline(); ok {
		t.conn.SetReadDeadline(deadline)
	}
	
	messageType, data, err := t.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("WebSocket read failed: %w", err)
	}
	
	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected message type: %d", messageType)
	}
	
	return data, nil
}

func (t *WebSocketTransport) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	if t.conn != nil && !t.closed {
		t.closed = true
		return t.conn.Close()
	}
	return nil
}

func (t *WebSocketTransport) IsConnected() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.conn != nil && !t.closed
}

func (t *WebSocketTransport) GetType() string {
	return "websocket"
}

// TransportManager manages multiple transport connections
type TransportManager struct {
	transports map[string]Transport
	mutex      sync.RWMutex
}

// NewTransportManager creates a new transport manager
func NewTransportManager() *TransportManager {
	return &TransportManager{
		transports: make(map[string]Transport),
	}
}

func (tm *TransportManager) AddTransport(name string, transport Transport) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.transports[name] = transport
}

func (tm *TransportManager) GetTransport(name string) (Transport, bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	transport, exists := tm.transports[name]
	return transport, exists
}

func (tm *TransportManager) RemoveTransport(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if transport, exists := tm.transports[name]; exists {
		err := transport.Close()
		delete(tm.transports, name)
		return err
	}
	return nil
}

func (tm *TransportManager) CloseAll() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	var errors []error
	for name, transport := range tm.transports {
		if err := transport.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close %s: %w", name, err))
		}
	}
	
	tm.transports = make(map[string]Transport)
	
	if len(errors) > 0 {
		return fmt.Errorf("transport close errors: %v", errors)
	}
	return nil
}

func (tm *TransportManager) ListTransports() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	names := make([]string, 0, len(tm.transports))
	for name := range tm.transports {
		names = append(names, name)
	}
	return names
}

// Message represents a transport message
type Message struct {
	ID        string                 ` + "`json:\"id\"`" + `
	Type      string                 ` + "`json:\"type\"`" + `
	Payload   map[string]interface{} ` + "`json:\"payload\"`" + `
	Timestamp time.Time              ` + "`json:\"timestamp\"`" + `
	Source    string                 ` + "`json:\"source\"`" + `
}

// BroadcastMessage sends a message to all transports
func (tm *TransportManager) BroadcastMessage(ctx context.Context, message Message) error {
	tm.mutex.RLock()
	transports := make(map[string]Transport, len(tm.transports))
	for name, transport := range tm.transports {
		transports[name] = transport
	}
	tm.mutex.RUnlock()
	
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	g, ctx := errgroup.WithContext(ctx)
	
	for name, transport := range transports {
		name, transport := name, transport // capture loop variables
		g.Go(func() error {
			if !transport.IsConnected() {
				return fmt.Errorf("transport %s not connected", name)
			}
			
			return transport.Send(ctx, data)
		})
	}
	
	return g.Wait()
}

// LoadBalancer distributes messages across multiple transports
type LoadBalancer struct {
	transports []Transport
	current    int
	mutex      sync.Mutex
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(transports []Transport) *LoadBalancer {
	return &LoadBalancer{
		transports: transports,
	}
}

func (lb *LoadBalancer) NextTransport() Transport {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	
	if len(lb.transports) == 0 {
		return nil
	}
	
	transport := lb.transports[lb.current]
	lb.current = (lb.current + 1) % len(lb.transports)
	return transport
}

func (lb *LoadBalancer) SendBalanced(ctx context.Context, data []byte) error {
	transport := lb.NextTransport()
	if transport == nil {
		return fmt.Errorf("no transports available")
	}
	
	if !transport.IsConnected() {
		return fmt.Errorf("selected transport not connected")
	}
	
	return transport.Send(ctx, data)
}

func main() {
	ctx := context.Background()
	
	// Create transport manager
	manager := NewTransportManager()
	defer manager.CloseAll()
	
	// Add HTTP transport
	httpTransport := NewHTTPTransport("http://localhost:8080")
	manager.AddTransport("http", httpTransport)
	
	// Add WebSocket transport
	wsTransport := NewWebSocketTransport("ws://localhost:8081/ws")
	if err := wsTransport.Connect(ctx); err != nil {
		log.Printf("Failed to connect WebSocket: %v", err)
	} else {
		manager.AddTransport("websocket", wsTransport)
	}
	
	// Test message broadcasting
	message := Message{
		ID:   "test-1",
		Type: "ping",
		Payload: map[string]interface{}{
			"message": "Hello from transport test",
		},
		Timestamp: time.Now(),
		Source:    "go-transport-test",
	}
	
	if err := manager.BroadcastMessage(ctx, message); err != nil {
		log.Printf("Broadcast failed: %v", err)
	} else {
		log.Println("Message broadcast successful")
	}
	
	// List active transports
	transports := manager.ListTransports()
	log.Printf("Active transports: %v", transports)
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "transport.go"), []byte(transportGo), 0644))
	
	// Create Python project for transport testing
	pythonProject := filepath.Join(suite.workspaceRoot, "python-transport-test")
	suite.Require().NoError(os.MkdirAll(pythonProject, 0755))
	
	requirementsTxt := `asyncio-mqtt==0.13.0
websockets==12.0
aiohttp==3.9.1
pydantic==2.5.0
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "requirements.txt"), []byte(requirementsTxt), 0644))
	
	asyncTransportPy := `"""Asynchronous transport layer testing module."""

import asyncio
import json
import logging
import time
from abc import ABC, abstractmethod
from typing import Dict, List, Optional, Any
import websockets
import aiohttp
from pydantic import BaseModel

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class Message(BaseModel):
    """Transport message model."""
    id: str
    type: str
    payload: Dict[str, Any]
    timestamp: float
    source: str

class TransportProtocol(ABC):
    """Abstract base class for transport protocols."""
    
    @abstractmethod
    async def connect(self) -> bool:
        """Connect to the transport."""
        pass
    
    @abstractmethod
    async def disconnect(self) -> None:
        """Disconnect from the transport."""
        pass
    
    @abstractmethod
    async def send(self, message: Message) -> bool:
        """Send a message via the transport."""
        pass
    
    @abstractmethod
    async def receive(self, timeout: float = 10.0) -> Optional[Message]:
        """Receive a message via the transport."""
        pass
    
    @abstractmethod
    def is_connected(self) -> bool:
        """Check if transport is connected."""
        pass
    
    @abstractmethod
    def get_type(self) -> str:
        """Get transport type identifier."""
        pass

class HTTPTransport(TransportProtocol):
    """HTTP-based transport implementation."""
    
    def __init__(self, base_url: str, timeout: float = 30.0):
        self.base_url = base_url.rstrip('/')
        self.timeout = aiohttp.ClientTimeout(total=timeout)
        self.session: Optional[aiohttp.ClientSession] = None
        self._connected = False
    
    async def connect(self) -> bool:
        """Establish HTTP session."""
        try:
            self.session = aiohttp.ClientSession(timeout=self.timeout)
            # Test connection with health check
            async with self.session.get(f"{self.base_url}/health") as response:
                if response.status == 200:
                    self._connected = True
                    logger.info("HTTP transport connected")
                    return True
                else:
                    logger.warning(f"HTTP health check failed: {response.status}")
                    return False
        except Exception as e:
            logger.error(f"HTTP connection failed: {e}")
            return False
    
    async def disconnect(self) -> None:
        """Close HTTP session."""
        if self.session:
            await self.session.close()
            self.session = None
        self._connected = False
        logger.info("HTTP transport disconnected")
    
    async def send(self, message: Message) -> bool:
        """Send message via HTTP POST."""
        if not self.session or not self._connected:
            logger.error("HTTP transport not connected")
            return False
        
        try:
            data = message.model_dump_json()
            async with self.session.post(
                f"{self.base_url}/message",
                data=data,
                headers={"Content-Type": "application/json"}
            ) as response:
                if response.status == 200:
                    logger.debug(f"HTTP message sent: {message.id}")
                    return True
                else:
                    logger.error(f"HTTP send failed: {response.status}")
                    return False
        except Exception as e:
            logger.error(f"HTTP send error: {e}")
            return False
    
    async def receive(self, timeout: float = 10.0) -> Optional[Message]:
        """Receive message via HTTP GET (polling)."""
        if not self.session or not self._connected:
            logger.error("HTTP transport not connected")
            return None
        
        try:
            async with self.session.get(
                f"{self.base_url}/message",
                timeout=aiohttp.ClientTimeout(total=timeout)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    message = Message(**data)
                    logger.debug(f"HTTP message received: {message.id}")
                    return message
                elif response.status == 204:
                    # No content available
                    return None
                else:
                    logger.error(f"HTTP receive failed: {response.status}")
                    return None
        except asyncio.TimeoutError:
            logger.debug("HTTP receive timeout")
            return None
        except Exception as e:
            logger.error(f"HTTP receive error: {e}")
            return None
    
    def is_connected(self) -> bool:
        return self._connected and self.session is not None
    
    def get_type(self) -> str:
        return "http"

class WebSocketTransport(TransportProtocol):
    """WebSocket-based transport implementation."""
    
    def __init__(self, url: str):
        self.url = url
        self.websocket: Optional[websockets.WebSocketServerProtocol] = None
        self._connected = False
    
    async def connect(self) -> bool:
        """Connect to WebSocket server."""
        try:
            self.websocket = await websockets.connect(self.url)
            self._connected = True
            logger.info(f"WebSocket connected to {self.url}")
            return True
        except Exception as e:
            logger.error(f"WebSocket connection failed: {e}")
            return False
    
    async def disconnect(self) -> None:
        """Disconnect from WebSocket."""
        if self.websocket:
            await self.websocket.close()
            self.websocket = None
        self._connected = False
        logger.info("WebSocket disconnected")
    
    async def send(self, message: Message) -> bool:
        """Send message via WebSocket."""
        if not self.websocket or not self._connected:
            logger.error("WebSocket transport not connected")
            return False
        
        try:
            data = message.model_dump_json()
            await self.websocket.send(data)
            logger.debug(f"WebSocket message sent: {message.id}")
            return True
        except Exception as e:
            logger.error(f"WebSocket send error: {e}")
            self._connected = False
            return False
    
    async def receive(self, timeout: float = 10.0) -> Optional[Message]:
        """Receive message via WebSocket."""
        if not self.websocket or not self._connected:
            logger.error("WebSocket transport not connected")
            return None
        
        try:
            data = await asyncio.wait_for(self.websocket.recv(), timeout=timeout)
            message_data = json.loads(data)
            message = Message(**message_data)
            logger.debug(f"WebSocket message received: {message.id}")
            return message
        except asyncio.TimeoutError:
            logger.debug("WebSocket receive timeout")
            return None
        except Exception as e:
            logger.error(f"WebSocket receive error: {e}")
            self._connected = False
            return None
    
    def is_connected(self) -> bool:
        return self._connected and self.websocket is not None
    
    def get_type(self) -> str:
        return "websocket"

class TransportManager:
    """Manages multiple transport connections."""
    
    def __init__(self):
        self.transports: Dict[str, TransportProtocol] = {}
        self._lock = asyncio.Lock()
    
    async def add_transport(self, name: str, transport: TransportProtocol) -> bool:
        """Add and connect a transport."""
        async with self._lock:
            if await transport.connect():
                self.transports[name] = transport
                logger.info(f"Transport '{name}' added and connected")
                return True
            else:
                logger.error(f"Failed to connect transport '{name}'")
                return False
    
    async def remove_transport(self, name: str) -> bool:
        """Remove and disconnect a transport."""
        async with self._lock:
            if name in self.transports:
                transport = self.transports[name]
                await transport.disconnect()
                del self.transports[name]
                logger.info(f"Transport '{name}' removed")
                return True
            else:
                logger.warning(f"Transport '{name}' not found")
                return False
    
    async def broadcast_message(self, message: Message) -> Dict[str, bool]:
        """Broadcast message to all connected transports."""
        results = {}
        
        async with self._lock:
            transports = list(self.transports.items())
        
        for name, transport in transports:
            if transport.is_connected():
                success = await transport.send(message)
                results[name] = success
                if success:
                    logger.info(f"Message sent via {name}")
                else:
                    logger.error(f"Failed to send via {name}")
            else:
                results[name] = False
                logger.warning(f"Transport {name} not connected")
        
        return results
    
    async def send_to_transport(self, transport_name: str, message: Message) -> bool:
        """Send message to specific transport."""
        async with self._lock:
            if transport_name in self.transports:
                transport = self.transports[transport_name]
                if transport.is_connected():
                    return await transport.send(message)
                else:
                    logger.error(f"Transport {transport_name} not connected")
                    return False
            else:
                logger.error(f"Transport {transport_name} not found")
                return False
    
    async def receive_from_all(self, timeout: float = 5.0) -> Dict[str, Optional[Message]]:
        """Receive messages from all transports."""
        results = {}
        
        async with self._lock:
            transports = list(self.transports.items())
        
        tasks = []
        for name, transport in transports:
            if transport.is_connected():
                task = asyncio.create_task(transport.receive(timeout))
                tasks.append((name, task))
        
        if not tasks:
            return results
        
        # Wait for all tasks to complete
        for name, task in tasks:
            try:
                message = await task
                results[name] = message
            except Exception as e:
                logger.error(f"Receive error from {name}: {e}")
                results[name] = None
        
        return results
    
    def get_connected_transports(self) -> List[str]:
        """Get list of connected transport names."""
        return [name for name, transport in self.transports.items() 
                if transport.is_connected()]
    
    def get_transport_status(self) -> Dict[str, Dict[str, Any]]:
        """Get status of all transports."""
        status = {}
        for name, transport in self.transports.items():
            status[name] = {
                "type": transport.get_type(),
                "connected": transport.is_connected()
            }
        return status
    
    async def close_all(self) -> None:
        """Close all transports."""
        async with self._lock:
            tasks = []
            for name, transport in self.transports.items():
                task = asyncio.create_task(transport.disconnect())
                tasks.append((name, task))
            
            for name, task in tasks:
                try:
                    await task
                    logger.info(f"Transport {name} closed")
                except Exception as e:
                    logger.error(f"Error closing {name}: {e}")
            
            self.transports.clear()

class LoadBalancer:
    """Load balancer for distributing messages across transports."""
    
    def __init__(self, transport_manager: TransportManager):
        self.transport_manager = transport_manager
        self.current_index = 0
    
    async def send_balanced(self, message: Message) -> bool:
        """Send message using round-robin load balancing."""
        connected_transports = self.transport_manager.get_connected_transports()
        
        if not connected_transports:
            logger.error("No connected transports available")
            return False
        
        # Round-robin selection
        transport_name = connected_transports[self.current_index % len(connected_transports)]
        self.current_index += 1
        
        success = await self.transport_manager.send_to_transport(transport_name, message)
        logger.info(f"Load balanced message sent via {transport_name}: {success}")
        return success

async def run_transport_test():
    """Run comprehensive transport layer test."""
    logger.info("Starting transport layer test...")
    
    # Create transport manager
    manager = TransportManager()
    
    # Add HTTP transport
    http_transport = HTTPTransport("http://localhost:8080")
    await manager.add_transport("http", http_transport)
    
    # Add WebSocket transport
    ws_transport = WebSocketTransport("ws://localhost:8081/ws")
    await manager.add_transport("websocket", ws_transport)
    
    # Create test message
    test_message = Message(
        id="test-msg-001",
        type="test",
        payload={
            "content": "Transport layer test message",
            "sequence": 1
        },
        timestamp=time.time(),
        source="python-transport-test"
    )
    
    # Test broadcasting
    logger.info("Testing message broadcasting...")
    broadcast_results = await manager.broadcast_message(test_message)
    logger.info(f"Broadcast results: {broadcast_results}")
    
    # Test load balancing
    logger.info("Testing load balancing...")
    load_balancer = LoadBalancer(manager)
    
    for i in range(5):
        message = Message(
            id=f"load-test-{i}",
            type="load_test",
            payload={"iteration": i},
            timestamp=time.time(),
            source="python-transport-test"
        )
        success = await load_balancer.send_balanced(message)
        logger.info(f"Load balanced message {i}: {success}")
        await asyncio.sleep(0.5)
    
    # Test receiving messages
    logger.info("Testing message receiving...")
    messages = await manager.receive_from_all(timeout=2.0)
    for transport_name, message in messages.items():
        if message:
            logger.info(f"Received from {transport_name}: {message.id}")
        else:
            logger.info(f"No message from {transport_name}")
    
    # Get transport status
    status = manager.get_transport_status()
    logger.info(f"Transport status: {status}")
    
    # Clean up
    await manager.close_all()
    logger.info("Transport test completed")

if __name__ == "__main__":
    asyncio.run(run_transport_test())
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "async_transport.py"), []byte(asyncTransportPy), 0644))
}

func (suite *TransportProtocolTestSuite) setupTransportConfigs() {
	// Configure STDIO transport for Go
	suite.configs["gopls-stdio"] = transport.ClientConfig{
		Command:   "gopls",
		Args:      []string{},
		Transport: transport.TransportStdio,
	}
	
	// Configure TCP transport (mock)
	suite.configs["gopls-tcp"] = transport.ClientConfig{
		Command:   "gopls",
		Args:      []string{"-listen", "tcp:127.0.0.1:0"},
		Transport: transport.TransportTCP,
	}
	
	// Configure STDIO transport for Python
	suite.configs["pylsp-stdio"] = transport.ClientConfig{
		Command:   "pylsp",
		Args:      []string{},
		Transport: transport.TransportStdio,
	}
}

// Test STDIO transport client creation and lifecycle
func (suite *TransportProtocolTestSuite) TestStdioTransportLifecycle() {
	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Second)
	defer cancel()

	testCases := []struct {
		name       string
		configName string
		language   string
	}{
		{
			name:       "Go STDIO transport",
			configName: "gopls-stdio",
			language:   "go",
		},
		{
			name:       "Python STDIO transport", 
			configName: "pylsp-stdio",
			language:   "python",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			config, exists := suite.configs[tc.configName]
			suite.True(exists, "Config should exist")

			// Create LSP client
			client, err := transport.NewLSPClient(config)
			suite.NoError(err, "Should create LSP client")
			suite.NotNil(client, "Client should not be nil")

			// Store client for cleanup
			suite.clientMutex.Lock()
			suite.clients[tc.configName] = client
			suite.clientMutex.Unlock()

			// Test client initialization
			suite.False(client.IsActive(), "Client should not be active initially")

			// Start client
			err = client.Start(ctx)
			suite.NoError(err, "Should start client successfully")

			// Verify client is active
			suite.True(client.IsActive(), "Client should be active after start")

			// Test basic LSP request
			params := map[string]interface{}{
				"processId": nil,
				"clientInfo": map[string]interface{}{
					"name":    "transport-test",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{},
			}

			// Send initialize request
			result, err := client.SendRequest(ctx, "initialize", params)
			suite.NoError(err, "Initialize request should succeed")
			suite.NotNil(result, "Initialize result should not be nil")

			// Send initialized notification
			err = client.SendNotification(ctx, "initialized", map[string]interface{}{})
			suite.NoError(err, "Initialized notification should succeed")

			// Test client stop
			err = client.Stop()
			suite.NoError(err, "Should stop client successfully")
			suite.False(client.IsActive(), "Client should not be active after stop")
		})
	}
}

// Test TCP transport client creation (mock scenario)
func (suite *TransportProtocolTestSuite) TestTCPTransportCreation() {
	config := suite.configs["gopls-tcp"]
	
	// Create TCP client (will fail to connect, but should create successfully)
	client, err := transport.NewLSPClient(config)
	
	// TCP transport creation should succeed even if connection fails
	suite.NoError(err, "Should create TCP client")
	suite.NotNil(client, "TCP client should not be nil")
	
	// Verify transport type
	// Note: This is a creation test, not a connection test
	suite.False(client.IsActive(), "TCP client should not be active without connection")
}

// Test multiple client instances with workspace isolation
func (suite *TransportProtocolTestSuite) TestMultipleClientInstances() {
	ctx, cancel := context.WithTimeout(suite.ctx, 45*time.Second)
	defer cancel()

	// Create multiple clients for different projects
	configs := []struct {
		name    string
		config  transport.ClientConfig
		project string
	}{
		{
			name:    "go-project-1",
			config:  suite.configs["gopls-stdio"],
			project: filepath.Join(suite.workspaceRoot, "go-transport-test"),
		},
		{
			name:    "python-project-1",
			config:  suite.configs["pylsp-stdio"],
			project: filepath.Join(suite.workspaceRoot, "python-transport-test"),
		},
	}

	// Start all clients
	for _, cfg := range configs {
		suite.Run(fmt.Sprintf("Start client %s", cfg.name), func() {
			client, err := transport.NewLSPClient(cfg.config)
			suite.NoError(err, "Should create client")
			suite.NotNil(client, "Client should not be nil")

			// Store client
			suite.clientMutex.Lock()
			suite.clients[cfg.name] = client
			suite.clientMutex.Unlock()

			// Start client
			err = client.Start(ctx)
			suite.NoError(err, "Should start client")
			suite.True(client.IsActive(), "Client should be active")
		})
	}

	// Test concurrent requests across clients
	suite.Run("Concurrent requests", func() {
		var wg sync.WaitGroup
		results := make(map[string]error)
		resultsMutex := sync.Mutex{}

		for name, client := range suite.clients {
			if !client.IsActive() {
				continue
			}

			wg.Add(1)
			go func(clientName string, c transport.LSPClient) {
				defer wg.Done()

				// Send initialize request
				params := map[string]interface{}{
					"processId": nil,
					"clientInfo": map[string]interface{}{
						"name":    fmt.Sprintf("transport-test-%s", clientName),
						"version": "1.0.0",
					},
					"capabilities": map[string]interface{}{},
				}

				_, err := c.SendRequest(ctx, "initialize", params)
				
				resultsMutex.Lock()
				results[clientName] = err
				resultsMutex.Unlock()
			}(name, client)
		}

		wg.Wait()

		// Verify all requests succeeded
		for clientName, err := range results {
			suite.NoError(err, fmt.Sprintf("Client %s request should succeed", clientName))
		}
	})

	// Clean up all clients
	suite.clientMutex.Lock()
	for name, client := range suite.clients {
		if client.IsActive() {
			client.Stop()
		}
	}
	suite.clientMutex.Unlock()
}

// Test transport layer switching between clients
func (suite *TransportProtocolTestSuite) TestTransportLayerSwitching() {
	ctx, cancel := context.WithTimeout(suite.ctx, 60*time.Second)
	defer cancel()

	// Test switching between different transport types
	transports := []struct {
		name       string
		configName string
		delay      time.Duration
	}{
		{
			name:       "stdio-1",
			configName: "gopls-stdio",
			delay:      100 * time.Millisecond,
		},
		{
			name:       "stdio-2", 
			configName: "pylsp-stdio",
			delay:      200 * time.Millisecond,
		},
	}

	// Test rapid switching between transports
	for i := 0; i < 3; i++ {
		for _, transport := range transports {
			suite.Run(fmt.Sprintf("Switch to %s (iteration %d)", transport.name, i+1), func() {
				config := suite.configs[transport.configName]
				
				start := time.Now()
				
				// Create client
				client, err := transport.NewLSPClient(config)
				suite.NoError(err, "Should create client")

				// Start client
				err = client.Start(ctx)
				suite.NoError(err, "Should start client")
				suite.True(client.IsActive(), "Client should be active")

				// Send test request
				params := map[string]interface{}{
					"processId":    nil,
					"capabilities": map[string]interface{}{},
				}

				_, err = client.SendRequest(ctx, "initialize", params)
				suite.NoError(err, "Request should succeed")

				// Stop client
				err = client.Stop()
				suite.NoError(err, "Should stop client")

				duration := time.Since(start)
				
				// Verify transport switching is fast (should complete within 1 second)
				suite.True(duration < 1*time.Second, 
					fmt.Sprintf("Transport switch should be fast, took %v", duration))

				// Small delay between switches
				time.Sleep(transport.delay)
			})
		}
	}
}

// Test transport protocol error handling
func (suite *TransportProtocolTestSuite) TestTransportProtocolErrorHandling() {
	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Second)
	defer cancel()

	testCases := []struct {
		name           string
		config         transport.ClientConfig
		expectStartErr bool
	}{
		{
			name: "Invalid command",
			config: transport.ClientConfig{
				Command:   "nonexistent-lsp-server",
				Args:      []string{},
				Transport: transport.TransportStdio,
			},
			expectStartErr: true,
		},
		{
			name: "Invalid transport type",
			config: transport.ClientConfig{
				Command:   "gopls",
				Args:      []string{},
				Transport: "invalid-transport",
			},
			expectStartErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			client, err := transport.NewLSPClient(tc.config)
			
			if tc.config.Transport == "invalid-transport" {
				// Should fail at client creation
				suite.Error(err, "Should fail to create client with invalid transport")
				suite.Nil(client, "Client should be nil")
				return
			}

			suite.NoError(err, "Should create client")
			suite.NotNil(client, "Client should not be nil")

			// Test start
			err = client.Start(ctx)
			if tc.expectStartErr {
				suite.Error(err, "Should fail to start client")
				suite.False(client.IsActive(), "Client should not be active")
			} else {
				suite.NoError(err, "Should start client")
				suite.True(client.IsActive(), "Client should be active")
				
				// Clean up
				client.Stop()
			}
		})
	}
}

// Test transport layer timeout handling
func (suite *TransportProtocolTestSuite) TestTransportTimeoutHandling() {
	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Second)
	defer cancel()

	config := suite.configs["gopls-stdio"]
	client, err := transport.NewLSPClient(config)
	suite.NoError(err, "Should create client")
	suite.NotNil(client, "Client should not be nil")

	// Start client
	err = client.Start(ctx)
	suite.NoError(err, "Should start client")
	suite.True(client.IsActive(), "Client should be active")

	defer client.Stop()

	// Test request with short timeout
	shortCtx, shortCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer shortCancel()

	// This request should timeout due to very short deadline
	params := map[string]interface{}{
		"processId":    nil,
		"capabilities": map[string]interface{}{},
	}

	_, err = client.SendRequest(shortCtx, "initialize", params)
	
	// Should either succeed quickly or timeout
	if err != nil {
		suite.Contains(err.Error(), "timeout", "Error should be timeout-related")
	}

	// Test request with reasonable timeout
	normalCtx, normalCancel := context.WithTimeout(ctx, 10*time.Second)
	defer normalCancel()

	_, err = client.SendRequest(normalCtx, "initialize", params)
	suite.NoError(err, "Request with normal timeout should succeed")
}

// Test concurrent transport operations
func (suite *TransportProtocolTestSuite) TestConcurrentTransportOperations() {
	ctx, cancel := context.WithTimeout(suite.ctx, 60*time.Second)
	defer cancel()

	config := suite.configs["gopls-stdio"]
	client, err := transport.NewLSPClient(config)
	suite.NoError(err, "Should create client")

	err = client.Start(ctx)
	suite.NoError(err, "Should start client")
	defer client.Stop()

	// Initialize client first
	params := map[string]interface{}{
		"processId":    nil,
		"capabilities": map[string]interface{}{},
	}

	_, err = client.SendRequest(ctx, "initialize", params)
	suite.NoError(err, "Initialize should succeed")

	err = client.SendNotification(ctx, "initialized", map[string]interface{}{})
	suite.NoError(err, "Initialized notification should succeed")

	// Test concurrent requests and notifications
	const numOperations = 10
	var wg sync.WaitGroup
	errors := make([]error, numOperations*2) // requests + notifications

	// Start concurrent operations
	for i := 0; i < numOperations; i++ {
		// Concurrent requests
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			requestParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file:///test%d.go", index),
				},
			}
			
			_, err := client.SendRequest(ctx, "textDocument/documentSymbol", requestParams)
			errors[index] = err
		}(i)

		// Concurrent notifications
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			notifParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        fmt.Sprintf("file:///test%d.go", index),
					"languageId": "go",
					"version":    1,
					"text":       fmt.Sprintf("package test%d", index),
				},
			}
			
			err := client.SendNotification(ctx, "textDocument/didOpen", notifParams)
			errors[numOperations+index] = err
		}(i)
	}

	wg.Wait()

	// Verify all operations completed successfully
	for i, err := range errors {
		if err != nil {
			suite.NoError(err, fmt.Sprintf("Operation %d should succeed", i))
		}
	}
}

// Test transport performance requirements
func (suite *TransportProtocolTestSuite) TestTransportPerformanceRequirements() {
	ctx, cancel := context.WithTimeout(suite.ctx, 45*time.Second)
	defer cancel()

	testCases := []struct {
		name      string
		operation string
		timeout   time.Duration
	}{
		{
			name:      "Client startup performance",
			operation: "start",
			timeout:   5 * time.Second,
		},
		{
			name:      "LSP request performance",
			operation: "request",
			timeout:   2 * time.Second,
		},
		{
			name:      "Client shutdown performance",
			operation: "stop",
			timeout:   1 * time.Second,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			config := suite.configs["gopls-stdio"]
			client, err := transport.NewLSPClient(config)
			suite.NoError(err, "Should create client")

			switch tc.operation {
			case "start":
				start := time.Now()
				err = client.Start(ctx)
				duration := time.Since(start)
				
				suite.NoError(err, "Client start should succeed")
				suite.True(duration < tc.timeout, 
					fmt.Sprintf("Client start should complete within %v, took %v", tc.timeout, duration))
				
				defer client.Stop()

			case "request":
				// Start client first
				err = client.Start(ctx)
				suite.NoError(err, "Should start client")
				defer client.Stop()

				// Initialize
				params := map[string]interface{}{
					"processId":    nil,
					"capabilities": map[string]interface{}{},
				}
				_, err = client.SendRequest(ctx, "initialize", params)
				suite.NoError(err, "Initialize should succeed")

				// Time the actual request
				start := time.Now()
				_, err = client.SendRequest(ctx, "textDocument/documentSymbol", map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test.go",
					},
				})
				duration := time.Since(start)

				suite.NoError(err, "Request should succeed")
				suite.True(duration < tc.timeout,
					fmt.Sprintf("Request should complete within %v, took %v", tc.timeout, duration))

			case "stop":
				// Start client first
				err = client.Start(ctx)
				suite.NoError(err, "Should start client")

				// Time the stop operation
				start := time.Now()
				err = client.Stop()
				duration := time.Since(start)

				suite.NoError(err, "Client stop should succeed")
				suite.True(duration < tc.timeout,
					fmt.Sprintf("Client stop should complete within %v, took %v", tc.timeout, duration))
			}
		})
	}
}