package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mcp "lsp-gateway/tests/mcp"
)

type MessageHandler struct {
	requestTracker   *RequestTracker
	logger           *TestLogger
	
	incomingCh       chan []byte
	outgoingCh       chan []byte
	errorCh          chan error
	shutdownCh       chan struct{}
	
	messageProcessor *MessageProcessor
	responseRouter   *ResponseRouter
	messageValidator *MessageValidator
	
	metrics          *MessageMetrics
	running          bool
	mu               sync.RWMutex
	shutdownOnce     sync.Once
}

type MessageProcessor struct {
	logger           *TestLogger
	maxMessageSize   int
	bufferSize       int
	processingTime   time.Duration
	mu               sync.RWMutex
}

type ResponseRouter struct {
	requestTracker   *RequestTracker
	logger           *TestLogger
	routingTable     map[string]chan *mcp.MCPResponse
	mu               sync.RWMutex
}

type MessageValidator struct {
	logger           *TestLogger
	validationRules  []ValidationRule
	metrics          *ValidationMetrics
	mu               sync.RWMutex
}

type ValidationRule struct {
	Name     string
	Check    func(*mcp.MCPMessage) error
	Severity ValidationSeverity
}

type ValidationSeverity int

const (
	ValidationError ValidationSeverity = iota
	ValidationWarning
	ValidationInfo
)

type MessageMetrics struct {
	MessagesSent      int64
	MessagesReceived  int64
	NotificationsSent int64
	RequestsSent      int64
	ResponsesReceived int64
	ProcessingErrors  int64
	ValidationErrors  int64
	AverageProcessingTime time.Duration
	LastMessageTime   time.Time
	mu                sync.RWMutex
}

type ValidationMetrics struct {
	TotalValidations int64
	ValidationErrors int64
	ValidationWarnings int64
	ErrorsByRule     map[string]int64
	mu               sync.RWMutex
}

const (
	MaxMessageSize     = 10 * 1024 * 1024 // 10MB
	DefaultBufferSize  = 1000
	ProcessingTimeout  = 30 * time.Second
)

func NewMessageHandler(requestTracker *RequestTracker, logger *TestLogger) *MessageHandler {
	mh := &MessageHandler{
		requestTracker: requestTracker,
		logger:         logger,
		incomingCh:     make(chan []byte, DefaultBufferSize),
		outgoingCh:     make(chan []byte, DefaultBufferSize),
		errorCh:        make(chan error, 100),
		shutdownCh:     make(chan struct{}),
		metrics:        &MessageMetrics{},
	}
	
	mh.messageProcessor = NewMessageProcessor(logger)
	mh.responseRouter = NewResponseRouter(requestTracker, logger)
	mh.messageValidator = NewMessageValidator(logger)
	
	return mh
}

func (mh *MessageHandler) Start() {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	
	if mh.running {
		return
	}
	
	mh.running = true
	go mh.messageLoop()
	mh.logger.Info("Message handler started")
}

func (mh *MessageHandler) SendRequest(ctx context.Context, request *mcp.MCPRequest) (*mcp.MCPResponse, error) {
	if !mh.isRunning() {
		return nil, fmt.Errorf("message handler not running")
	}
	
	start := time.Now()
	defer func() {
		processingTime := time.Since(start)
		mh.updateProcessingTime(processingTime)
	}()
	
	message := &mcp.MCPMessage{
		JSONRPC: request.JSONRPC,
		ID:      request.ID,
		Method:  request.Method,
		Params:  request.Params,
	}
	
	if err := mh.messageValidator.ValidateMessage(message); err != nil {
		mh.recordValidationError()
		return nil, fmt.Errorf("request validation failed: %w", err)
	}
	
	responseCh := make(chan *mcp.MCPResponse, 1)
	errorCh := make(chan error, 1)
	
	requestID := fmt.Sprintf("%v", request.ID)
	mh.responseRouter.RegisterRequest(requestID, responseCh)
	defer mh.responseRouter.UnregisterRequest(requestID)
	
	if err := mh.sendMessage(message); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	mh.recordRequestSent()
	
	select {
	case response := <-responseCh:
		mh.recordResponseReceived()
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-mh.shutdownCh:
		return nil, fmt.Errorf("message handler shutdown")
	}
}

func (mh *MessageHandler) SendNotification(ctx context.Context, notification *mcp.MCPMessage) error {
	if !mh.isRunning() {
		return fmt.Errorf("message handler not running")
	}
	
	if err := mh.messageValidator.ValidateMessage(notification); err != nil {
		mh.recordValidationError()
		return fmt.Errorf("notification validation failed: %w", err)
	}
	
	if err := mh.sendMessage(notification); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	
	mh.recordNotificationSent()
	return nil
}

func (mh *MessageHandler) sendMessage(message *mcp.MCPMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum %d", len(data), MaxMessageSize)
	}
	
	select {
	case mh.outgoingCh <- data:
		mh.recordMessageSent()
		return nil
	default:
		return fmt.Errorf("outgoing message buffer full")
	}
}

func (mh *MessageHandler) ProcessIncomingMessage(data []byte) error {
	if !mh.isRunning() {
		return fmt.Errorf("message handler not running")
	}
	
	select {
	case mh.incomingCh <- data:
		return nil
	default:
		return fmt.Errorf("incoming message buffer full")
	}
}

func (mh *MessageHandler) GetNextOutgoingMessage() ([]byte, error) {
	select {
	case data := <-mh.outgoingCh:
		return data, nil
	case <-time.After(100 * time.Millisecond):
		return nil, fmt.Errorf("no outgoing messages available")
	}
}

func (mh *MessageHandler) GetMetrics() *MessageMetrics {
	mh.metrics.mu.RLock()
	defer mh.metrics.mu.RUnlock()
	
	return &MessageMetrics{
		MessagesSent:      atomic.LoadInt64(&mh.metrics.MessagesSent),
		MessagesReceived:  atomic.LoadInt64(&mh.metrics.MessagesReceived),
		NotificationsSent: atomic.LoadInt64(&mh.metrics.NotificationsSent),
		RequestsSent:      atomic.LoadInt64(&mh.metrics.RequestsSent),
		ResponsesReceived: atomic.LoadInt64(&mh.metrics.ResponsesReceived),
		ProcessingErrors:  atomic.LoadInt64(&mh.metrics.ProcessingErrors),
		ValidationErrors:  atomic.LoadInt64(&mh.metrics.ValidationErrors),
		AverageProcessingTime: mh.metrics.AverageProcessingTime,
		LastMessageTime:   mh.metrics.LastMessageTime,
	}
}

func (mh *MessageHandler) Close() error {
	mh.shutdownOnce.Do(func() {
		close(mh.shutdownCh)
	})
	
	mh.mu.Lock()
	defer mh.mu.Unlock()
	
	mh.running = false
	mh.logger.Info("Message handler closed")
	return nil
}

func (mh *MessageHandler) messageLoop() {
	for {
		select {
		case data := <-mh.incomingCh:
			mh.processIncomingData(data)
		case <-mh.shutdownCh:
			return
		}
	}
}

func (mh *MessageHandler) processIncomingData(data []byte) {
	start := time.Now()
	defer func() {
		processingTime := time.Since(start)
		mh.updateProcessingTime(processingTime)
	}()
	
	message, err := mh.messageProcessor.ParseMessage(data)
	if err != nil {
		mh.recordProcessingError()
		mh.logger.Error("Failed to parse incoming message", "error", err)
		return
	}
	
	if err := mh.messageValidator.ValidateMessage(message); err != nil {
		mh.recordValidationError()
		mh.logger.Warn("Incoming message validation failed", "error", err)
	}
	
	mh.recordMessageReceived()
	
	if message.IsResponse() {
		mh.responseRouter.RouteResponse(message)
	} else if message.IsNotification() {
		mh.logger.Debug("Received notification", "method", message.Method)
	} else {
		mh.logger.Warn("Received unknown message type", "message", string(data))
	}
}

func (mh *MessageHandler) isRunning() bool {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.running
}

func (mh *MessageHandler) recordMessageSent() {
	atomic.AddInt64(&mh.metrics.MessagesSent, 1)
	mh.updateLastMessageTime()
}

func (mh *MessageHandler) recordMessageReceived() {
	atomic.AddInt64(&mh.metrics.MessagesReceived, 1)
	mh.updateLastMessageTime()
}

func (mh *MessageHandler) recordRequestSent() {
	atomic.AddInt64(&mh.metrics.RequestsSent, 1)
}

func (mh *MessageHandler) recordResponseReceived() {
	atomic.AddInt64(&mh.metrics.ResponsesReceived, 1)
}

func (mh *MessageHandler) recordNotificationSent() {
	atomic.AddInt64(&mh.metrics.NotificationsSent, 1)
}

func (mh *MessageHandler) recordProcessingError() {
	atomic.AddInt64(&mh.metrics.ProcessingErrors, 1)
}

func (mh *MessageHandler) recordValidationError() {
	atomic.AddInt64(&mh.metrics.ValidationErrors, 1)
}

func (mh *MessageHandler) updateProcessingTime(duration time.Duration) {
	mh.metrics.mu.Lock()
	defer mh.metrics.mu.Unlock()
	
	if mh.metrics.AverageProcessingTime == 0 {
		mh.metrics.AverageProcessingTime = duration
	} else {
		mh.metrics.AverageProcessingTime = (mh.metrics.AverageProcessingTime + duration) / 2
	}
}

func (mh *MessageHandler) updateLastMessageTime() {
	mh.metrics.mu.Lock()
	defer mh.metrics.mu.Unlock()
	mh.metrics.LastMessageTime = time.Now()
}

func NewMessageProcessor(logger *TestLogger) *MessageProcessor {
	return &MessageProcessor{
		logger:         logger,
		maxMessageSize: MaxMessageSize,
		bufferSize:     DefaultBufferSize,
	}
}

func (mp *MessageProcessor) ParseMessage(data []byte) (*mcp.MCPMessage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty message data")
	}
	
	if len(data) > mp.maxMessageSize {
		return nil, fmt.Errorf("message size %d exceeds maximum %d", len(data), mp.maxMessageSize)
	}
	
	var message mcp.MCPMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	return &message, nil
}

func NewResponseRouter(requestTracker *RequestTracker, logger *TestLogger) *ResponseRouter {
	return &ResponseRouter{
		requestTracker: requestTracker,
		logger:         logger,
		routingTable:   make(map[string]chan *mcp.MCPResponse),
	}
}

func (rr *ResponseRouter) RegisterRequest(requestID string, responseCh chan *mcp.MCPResponse) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.routingTable[requestID] = responseCh
}

func (rr *ResponseRouter) UnregisterRequest(requestID string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	delete(rr.routingTable, requestID)
}

func (rr *ResponseRouter) RouteResponse(message *mcp.MCPMessage) {
	if !message.IsResponse() {
		rr.logger.Warn("Attempted to route non-response message", "messageType", "unknown")
		return
	}
	
	requestID := fmt.Sprintf("%v", message.ID)
	
	rr.mu.RLock()
	responseCh, exists := rr.routingTable[requestID]
	rr.mu.RUnlock()
	
	if !exists {
		rr.logger.Warn("No route found for response", "requestID", requestID)
		return
	}
	
	response := &mcp.MCPResponse{
		JSONRPC: message.JSONRPC,
		ID:      message.ID,
		Result:  message.Result,
		Error:   message.Error,
	}
	
	select {
	case responseCh <- response:
		rr.logger.Debug("Response routed successfully", "requestID", requestID)
	default:
		rr.logger.Warn("Response channel full, dropping response", "requestID", requestID)
	}
}

func NewMessageValidator(logger *TestLogger) *MessageValidator {
	mv := &MessageValidator{
		logger:  logger,
		metrics: &ValidationMetrics{
			ErrorsByRule: make(map[string]int64),
		},
	}
	
	mv.validationRules = []ValidationRule{
		{
			Name:     "JSONRPCVersion",
			Check:    mv.validateJSONRPCVersion,
			Severity: ValidationError,
		},
		{
			Name:     "MessageStructure",
			Check:    mv.validateMessageStructure,
			Severity: ValidationError,
		},
		{
			Name:     "RequestID",
			Check:    mv.validateRequestID,
			Severity: ValidationWarning,
		},
	}
	
	return mv
}

func (mv *MessageValidator) ValidateMessage(message *mcp.MCPMessage) error {
	mv.metrics.mu.Lock()
	atomic.AddInt64(&mv.metrics.TotalValidations, 1)
	mv.metrics.mu.Unlock()
	
	var errors []string
	var warnings []string
	
	for _, rule := range mv.validationRules {
		if err := rule.Check(message); err != nil {
			mv.recordRuleViolation(rule.Name)
			
			switch rule.Severity {
			case ValidationError:
				errors = append(errors, fmt.Sprintf("%s: %v", rule.Name, err))
			case ValidationWarning:
				warnings = append(warnings, fmt.Sprintf("%s: %v", rule.Name, err))
			}
		}
	}
	
	if len(warnings) > 0 {
		mv.recordValidationWarning()
		mv.logger.Debug("Message validation warnings", "warnings", warnings)
	}
	
	if len(errors) > 0 {
		mv.recordValidationError()
		return fmt.Errorf("validation errors: %v", errors)
	}
	
	return nil
}

func (mv *MessageValidator) validateJSONRPCVersion(message *mcp.MCPMessage) error {
	if message.JSONRPC != mcp.JSONRPCVersion {
		return fmt.Errorf("invalid JSON-RPC version: expected %s, got %s", mcp.JSONRPCVersion, message.JSONRPC)
	}
	return nil
}

func (mv *MessageValidator) validateMessageStructure(message *mcp.MCPMessage) error {
	if message.IsRequest() {
		if message.Method == "" {
			return fmt.Errorf("request missing method field")
		}
		if message.ID == nil {
			return fmt.Errorf("request missing ID field")
		}
	} else if message.IsResponse() {
		if message.ID == nil {
			return fmt.Errorf("response missing ID field")
		}
		if message.Result == nil && message.Error == nil {
			return fmt.Errorf("response missing both result and error fields")
		}
	} else if message.IsNotification() {
		if message.Method == "" {
			return fmt.Errorf("notification missing method field")
		}
	} else {
		return fmt.Errorf("unknown message type")
	}
	return nil
}

func (mv *MessageValidator) validateRequestID(message *mcp.MCPMessage) error {
	if message.ID != nil {
		switch id := message.ID.(type) {
		case string:
			if len(id) == 0 {
				return fmt.Errorf("empty string request ID")
			}
		case float64:
			if id < 0 {
				return fmt.Errorf("negative numeric request ID")
			}
		default:
			return fmt.Errorf("invalid request ID type: %T", id)
		}
	}
	return nil
}

func (mv *MessageValidator) GetMetrics() *ValidationMetrics {
	mv.metrics.mu.RLock()
	defer mv.metrics.mu.RUnlock()
	
	errorsByRule := make(map[string]int64)
	for rule, count := range mv.metrics.ErrorsByRule {
		errorsByRule[rule] = count
	}
	
	return &ValidationMetrics{
		TotalValidations:   atomic.LoadInt64(&mv.metrics.TotalValidations),
		ValidationErrors:   atomic.LoadInt64(&mv.metrics.ValidationErrors),
		ValidationWarnings: atomic.LoadInt64(&mv.metrics.ValidationWarnings),
		ErrorsByRule:       errorsByRule,
	}
}

func (mv *MessageValidator) recordValidationError() {
	atomic.AddInt64(&mv.metrics.ValidationErrors, 1)
}

func (mv *MessageValidator) recordValidationWarning() {
	atomic.AddInt64(&mv.metrics.ValidationWarnings, 1)
}

func (mv *MessageValidator) recordRuleViolation(ruleName string) {
	mv.metrics.mu.Lock()
	defer mv.metrics.mu.Unlock()
	mv.metrics.ErrorsByRule[ruleName]++
}