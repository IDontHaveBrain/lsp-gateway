package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProtocolDetection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ProtocolFormat
		wantErr  bool
	}{
		{
			name:     "JSON format with jsonrpc field",
			input:    `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n",
			expected: ProtocolFormatJSON,
			wantErr:  false,
		},
		{
			name:     "JSON format with method field",
			input:    `{"method":"tools/list","id":2}` + "\n",
			expected: ProtocolFormatJSON,
			wantErr:  false,
		},
		{
			name:     "LSP Content-Length format",
			input:    "Content-Length: 45\r\n\r\n" + `{"jsonrpc":"2.0","method":"initialize","id":1}`,
			expected: ProtocolFormatLSP,
			wantErr:  false,
		},
		{
			name:     "LSP Content-Length case insensitive",
			input:    "content-length: 45\r\n\r\n" + `{"jsonrpc":"2.0","method":"initialize","id":1}`,
			expected: ProtocolFormatLSP,
			wantErr:  false,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: ProtocolFormatUnknown,
			wantErr:  true,
		},
		{
			name:     "Invalid JSON start",
			input:    `{invalid json}` + "\n",
			expected: ProtocolFormatJSON,
			wantErr:  false,
		},
		{
			name:     "Plain text that looks like headers",
			input:    "Some-Header: value\r\n\r\nContent here",
			expected: ProtocolFormatLSP,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(nil)
			
			if tt.input == "" {
				result, err := server.detectProtocolFormat([]byte{})
				if (err != nil) != tt.wantErr {
					t.Errorf("detectProtocolFormat() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if result != tt.expected {
					t.Errorf("detectProtocolFormat() = %v, want %v", result, tt.expected)
				}
				return
			}

			headerData := []byte(tt.input)
			if len(headerData) > 32 {
				headerData = headerData[:32]
			}

			result, err := server.detectProtocolFormat(headerData)
			if (err != nil) != tt.wantErr {
				t.Errorf("detectProtocolFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("detectProtocolFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProtocolDetectionConcurrency(t *testing.T) {
	// Test that multiple concurrent protocol detections don't interfere with each other
	numGoroutines := 10
	var wg sync.WaitGroup
	results := make([]ProtocolFormat, numGoroutines)
	errors := make([]error, numGoroutines)

	jsonInput := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n"
	lspInput := "Content-Length: 45\r\n\r\n" + `{"jsonrpc":"2.0","method":"initialize","id":1}`

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Create separate server instance for each goroutine to simulate separate connections
			server := NewServer(nil)
			
			var input string
			var expected ProtocolFormat
			if index%2 == 0 {
				input = jsonInput
				expected = ProtocolFormatJSON
			} else {
				input = lspInput
				expected = ProtocolFormatLSP
			}

			reader := bufio.NewReader(strings.NewReader(input))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			message, err := server.readMessageWithProtocolDetection(ctx, reader)
			if err != nil {
				errors[index] = err
				return
			}

			format, detected := server.connContext.GetProtocolFormat()
			if !detected {
				errors[index] = fmt.Errorf("protocol not detected")
				return
			}

			results[index] = format
			if format != expected {
				errors[index] = fmt.Errorf("expected %v, got %v", expected, format)
				return
			}

			// Verify message was read correctly
			if len(message) == 0 {
				errors[index] = fmt.Errorf("empty message")
				return
			}
		}(i)
	}

	wg.Wait()

	// Check results
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}

	// Verify all JSON inputs detected as JSON and all LSP inputs detected as LSP
	for i, result := range results {
		var expected ProtocolFormat
		if i%2 == 0 {
			expected = ProtocolFormatJSON
		} else {
			expected = ProtocolFormatLSP
		}
		
		if errors[i] == nil && result != expected {
			t.Errorf("Goroutine %d: expected %v, got %v", i, expected, result)
		}
	}
}

func TestConnectionContextThreadSafety(t *testing.T) {
	ctx := &ConnectionContext{}
	numGoroutines := 100
	var wg sync.WaitGroup

	// Test concurrent writes and reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Alternate between setting different formats
			if index%2 == 0 {
				ctx.SetProtocolFormat(ProtocolFormatJSON)
			} else {
				ctx.SetProtocolFormat(ProtocolFormatLSP)
			}
			
			// Read the format
			format, detected := ctx.GetProtocolFormat()
			if !detected {
				t.Errorf("Protocol format not detected in goroutine %d", index)
				return
			}
			
			if format != ProtocolFormatJSON && format != ProtocolFormatLSP {
				t.Errorf("Invalid protocol format %v in goroutine %d", format, index)
			}
		}(i)
	}

	wg.Wait()

	// Final check
	format, detected := ctx.GetProtocolFormat()
	if !detected {
		t.Error("Final protocol format not detected")
	}
	if format != ProtocolFormatJSON && format != ProtocolFormatLSP {
		t.Errorf("Final protocol format invalid: %v", format)
	}
}

func TestJSONMessageReading(t *testing.T) {
	server := NewServer(nil)
	
	testMessage := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n"
	reader := bufio.NewReader(strings.NewReader(testMessage))
	
	// Set protocol format to JSON
	server.connContext.SetProtocolFormat(ProtocolFormatJSON)
	
	ctx := context.Background()
	message, err := server.readJSONMessage(ctx, reader)
	if err != nil {
		t.Fatalf("readJSONMessage() error = %v", err)
	}
	
	expected := `{"jsonrpc":"2.0","method":"initialize","id":1}`
	if message != expected {
		t.Errorf("readJSONMessage() = %q, want %q", message, expected)
	}
}

func TestLSPMessageReading(t *testing.T) {
	server := NewServer(nil)
	
	content := `{"jsonrpc":"2.0","method":"initialize","id":1}`
	testMessage := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(content), content)
	reader := bufio.NewReader(strings.NewReader(testMessage))
	
	// Set protocol format to LSP
	server.connContext.SetProtocolFormat(ProtocolFormatLSP)
	
	ctx := context.Background()
	message, err := server.readLSPMessage(ctx, reader)
	if err != nil {
		t.Fatalf("readLSPMessage() error = %v", err)
	}
	
	if message != content {
		t.Errorf("readLSPMessage() = %q, want %q", message, content)
	}
}

func TestProtocolDetectionTimeout(t *testing.T) {
	server := NewServer(nil)
	
	// Create a reader that will block
	slowReader := &slowReader{delay: 10 * time.Second}
	reader := bufio.NewReader(slowReader)
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	_, err := server.peekWithTimeout(ctx, reader, 32)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestSendMessageWithProtocolFormat(t *testing.T) {
	tests := []struct {
		name           string
		protocolFormat ProtocolFormat
		detected       bool
		expectedPrefix string
	}{
		{
			name:           "JSON format",
			protocolFormat: ProtocolFormatJSON,
			detected:       true,
			expectedPrefix: `{"jsonrpc":"2.0"`,
		},
		{
			name:           "LSP format",
			protocolFormat: ProtocolFormatLSP,
			detected:       true,
			expectedPrefix: "Content-Length:",
		},
		{
			name:           "Undetected defaults to LSP",
			protocolFormat: ProtocolFormatUnknown,
			detected:       false,
			expectedPrefix: "Content-Length:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			server := NewServer(nil)
			server.Output = &output
			
			if tt.detected {
				server.connContext.SetProtocolFormat(tt.protocolFormat)
			}
			
			msg := MCPMessage{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "test",
			}
			
			err := server.SendMessage(msg)
			if err != nil {
				t.Fatalf("SendMessage() error = %v", err)
			}
			
			result := output.String()
			if !strings.HasPrefix(result, tt.expectedPrefix) {
				t.Errorf("SendMessage() output = %q, want prefix %q", result, tt.expectedPrefix)
			}
		})
	}
}

// slowReader is a helper for testing timeouts
type slowReader struct {
	delay time.Duration
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	time.Sleep(sr.delay)
	return 0, fmt.Errorf("slow reader timeout")
}