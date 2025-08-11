package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"testing"
)

type mockHandler struct {
	reqCount   int
	notifCount int
	respCount  int
	lastMethod string
	lastID     interface{}
	lastParams interface{}
	lastResult json.RawMessage
	lastErr    *RPCError
}

func (m *mockHandler) HandleRequest(method string, id interface{}, params interface{}) error {
	m.reqCount++
	m.lastMethod = method
	m.lastID = id
	m.lastParams = params
	return nil
}
func (m *mockHandler) HandleResponse(id interface{}, result json.RawMessage, err *RPCError) error {
	m.respCount++
	m.lastID = id
	m.lastResult = result
	m.lastErr = err
	return nil
}
func (m *mockHandler) HandleNotification(method string, params interface{}) error {
	m.notifCount++
	m.lastMethod = method
	m.lastParams = params
	return nil
}

func TestLSPJSONRPCProtocol_WriteMessage(t *testing.T) {
	p := NewLSPJSONRPCProtocol("go")
	buf := &bytes.Buffer{}
	msg := CreateMessage("initialize", 1, map[string]any{"capabilities": map[string]any{}})
	if err := p.WriteMessage(buf, msg); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}
	out := buf.String()
	if !contains(out, "Content-Length:") {
		t.Fatalf("missing Content-Length header: %q", out)
	}
	parts := bytes.SplitN(buf.Bytes(), []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		t.Fatalf("invalid header/body split: %q", out)
	}
	payload := parts[1]
	var dec JSONRPCMessage
	if err := json.Unmarshal(payload, &dec); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if dec.Method != "initialize" || dec.ID == nil {
		t.Fatalf("unexpected message decoded: %+v", dec)
	}
}

func TestLSPJSONRPCProtocol_HandleMessage_RoutesCorrectly(t *testing.T) {
	p := NewLSPJSONRPCProtocol("go")
	h := &mockHandler{}

	// Server request
	req := CreateMessage("workspace/configuration", 2, map[string]any{"items": []any{}})
	reqBytes, _ := json.Marshal(req)
	if err := p.HandleMessage(reqBytes, h); err != nil {
		t.Fatalf("handle request: %v", err)
	}
	if h.reqCount != 1 || h.lastMethod != "workspace/configuration" {
		t.Fatalf("request not handled: %+v", h)
	}

	// Server notification
	notif := CreateNotification("window/logMessage", map[string]any{"type": 3, "message": "x"})
	notifBytes, _ := json.Marshal(notif)
	if err := p.HandleMessage(notifBytes, h); err != nil {
		t.Fatalf("handle notification: %v", err)
	}
	if h.notifCount != 1 || h.lastMethod != "window/logMessage" {
		t.Fatalf("notification not handled: %+v", h)
	}

	// Client response success
	res := CreateResponse(2, map[string]any{"ok": true}, nil)
	resBytes, _ := json.Marshal(res)
	if err := p.HandleMessage(resBytes, h); err != nil {
		t.Fatalf("handle response: %v", err)
	}
	if h.respCount == 0 || h.lastErr != nil || len(h.lastResult) == 0 {
		t.Fatalf("response not handled as success: %+v", h)
	}

	// Client response error
	resErr := CreateResponse(3, nil, NewRPCError(InternalError, "boom", nil))
	resErrBytes, _ := json.Marshal(resErr)
	if err := p.HandleMessage(resErrBytes, h); err != nil {
		t.Fatalf("handle response error: %v", err)
	}
	if h.lastErr == nil || h.lastErr.Code != InternalError {
		t.Fatalf("expected internal error, got: %+v", h.lastErr)
	}

	// Malformed
	if err := p.HandleMessage([]byte(`{"jsonrpc":"2.0"}`), h); err == nil {
		t.Fatalf("expected error for malformed message")
	}
}

func TestLSPJSONRPCProtocol_HandleResponses_ParsesStream(t *testing.T) {
	p := NewLSPJSONRPCProtocol("go")
	h := &mockHandler{}

	// Build two messages in one stream
	msg1 := CreateResponse(1, map[string]any{"a": 1}, nil)
	b1, _ := json.Marshal(msg1)
	msg2 := CreateNotification("window/logMessage", map[string]any{"message": "hi"})
	b2, _ := json.Marshal(msg2)

	var stream bytes.Buffer
	stream.WriteString("Content-Length: ")
	stream.WriteString(intToString(len(b1)))
	stream.WriteString("\r\n\r\n")
	stream.Write(b1)
	stream.WriteString("Content-Length: ")
	stream.WriteString(intToString(len(b2)))
	stream.WriteString("\r\n\r\n")
	stream.Write(b2)

	stop := make(chan struct{})
	defer close(stop)
	if err := p.HandleResponses(bytes.NewReader(stream.Bytes()), h, stop); err != nil && err != io.EOF {
		t.Fatalf("HandleResponses error: %v", err)
	}
	if h.respCount == 0 || h.notifCount == 0 {
		t.Fatalf("expected both response and notification handled: %+v", h)
	}
}

func TestMessageHelpers(t *testing.T) {
	m := CreateMessage("initialize", 9, map[string]any{"x": true})
	if m.JSONRPC != JSONRPCVersion || m.Method != "initialize" || m.ID == nil {
		t.Fatalf("unexpected message: %+v", m)
	}
	n := CreateNotification("exit", nil)
	if n.Method != "exit" || n.ID != nil {
		t.Fatalf("unexpected notification: %+v", n)
	}
	e := NewRPCError(InvalidRequest, "oops", nil)
	r := CreateResponse(10, nil, e)
	if r.Error == nil || r.Error.Code != InvalidRequest {
		t.Fatalf("unexpected response error: %+v", r.Error)
	}
}

func intToString(v int) string { return strconv.Itoa(v) }
