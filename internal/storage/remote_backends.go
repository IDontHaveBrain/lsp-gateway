package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-redis/redis/v8"
)

// HTTPRestBackend implementation

func (h *HTTPRestBackend) GetBackendType() BackendType {
	return BackendCustom
}

func (h *HTTPRestBackend) GetEndpoint() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.endpoint
}

func (h *HTTPRestBackend) Initialize(ctx context.Context, config BackendConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.endpoint = config.ConnectionString
	h.timeout = time.Duration(config.TimeoutMs) * time.Millisecond
	
	// Setup authentication
	if config.APIKey != "" {
		h.headers["Authorization"] = "Bearer " + config.APIKey
	}
	if config.Username != "" && config.Password != "" {
		h.headers["Authorization"] = "Basic " + basicAuth(config.Username, config.Password)
	}

	// Test connection
	return h.Ping(ctx)
}

func (h *HTTPRestBackend) Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error) {
	url := fmt.Sprintf("%s/cache/%s", h.endpoint, url.PathEscape(key))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, ErrCacheEntryNotFound
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Extract metadata from headers
	metadata := &StorageMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: resp.Header.Get("Content-Type"),
		Encoding:    resp.Header.Get("Content-Encoding"),
		Compressed:  resp.Header.Get("Content-Encoding") != "",
		Checksum:    resp.Header.Get("ETag"),
	}

	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		if t, err := time.Parse(http.TimeFormat, lastModified); err == nil {
			metadata.ModifiedAt = t
		}
	}

	return data, metadata, nil
}

func (h *HTTPRestBackend) Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error {
	url := fmt.Sprintf("%s/cache/%s", h.endpoint, url.PathEscape(key))
	
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	// Add metadata headers
	if metadata != nil {
		req.Header.Set("Content-Type", metadata.ContentType)
		if metadata.Encoding != "" {
			req.Header.Set("Content-Encoding", metadata.Encoding)
		}
		if metadata.Checksum != "" {
			req.Header.Set("ETag", metadata.Checksum)
		}
		req.Header.Set("X-Cache-Version", strconv.FormatInt(metadata.Version, 10))
		req.Header.Set("X-Cache-Created", metadata.CreatedAt.Format(time.RFC3339))
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error: %d %s, body: %s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

func (h *HTTPRestBackend) Delete(ctx context.Context, key string) error {
	url := fmt.Sprintf("%s/cache/%s", h.endpoint, url.PathEscape(key))
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (h *HTTPRestBackend) Exists(ctx context.Context, key string) (bool, error) {
	url := fmt.Sprintf("%s/cache/%s", h.endpoint, url.PathEscape(key))
	
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	resp, err := h.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (h *HTTPRestBackend) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	// Use POST request with JSON body for batch get
	url := fmt.Sprintf("%s/cache/batch", h.endpoint)
	
	requestBody := map[string]interface{}{
		"operation": "get",
		"keys":      keys,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	if data, ok := response["data"].(map[string]interface{}); ok {
		for key, value := range data {
			if valueStr, ok := value.(string); ok {
				result[key] = []byte(valueStr)
			}
		}
	}

	return result, nil
}

func (h *HTTPRestBackend) PutBatch(ctx context.Context, entries map[string][]byte) error {
	// Use POST request with JSON body for batch put
	url := fmt.Sprintf("%s/cache/batch", h.endpoint)
	
	// Convert byte data to base64 for JSON transport
	data := make(map[string]string)
	for key, value := range entries {
		data[key] = string(value) // Simplified - should use base64 encoding
	}
	
	requestBody := map[string]interface{}{
		"operation": "put",
		"data":      data,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error: %d %s, body: %s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

func (h *HTTPRestBackend) DeleteBatch(ctx context.Context, keys []string) error {
	// Use POST request with JSON body for batch delete
	url := fmt.Sprintf("%s/cache/batch", h.endpoint)
	
	requestBody := map[string]interface{}{
		"operation": "delete",
		"keys":      keys,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (h *HTTPRestBackend) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", h.endpoint)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (h *HTTPRestBackend) GetStats(ctx context.Context) (*BackendStats, error) {
	url := fmt.Sprintf("%s/stats", h.endpoint)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	h.mu.RLock()
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.RUnlock()

	start := time.Now()
	resp, err := h.client.Do(req)
	responseTime := time.Since(start)
	
	if err != nil {
		return &BackendStats{
			Endpoint:     h.endpoint,
			Healthy:      false,
			ResponseTime: responseTime,
			LastCheck:    time.Now(),
		}, err
	}
	defer resp.Body.Close()

	stats := &BackendStats{
		Endpoint:     h.endpoint,
		Healthy:      resp.StatusCode == http.StatusOK,
		ResponseTime: responseTime,
		LastCheck:    time.Now(),
	}

	if resp.StatusCode == http.StatusOK {
		var serverStats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&serverStats); err == nil {
			if requests, ok := serverStats["requests"].(float64); ok {
				stats.RequestCount = int64(requests)
			}
			if bytes, ok := serverStats["bytes_transferred"].(float64); ok {
				stats.BytesTransferred = int64(bytes)
			}
			if errorRate, ok := serverStats["error_rate"].(float64); ok {
				stats.ErrorRate = errorRate
			}
		}
	}

	return stats, nil
}

func (h *HTTPRestBackend) Close() error {
	// Nothing to close for HTTP client
	return nil
}

// S3Backend implementation

func (s *S3Backend) GetBackendType() BackendType {
	return BackendS3
}

func (s *S3Backend) GetEndpoint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.endpoint
}

func (s *S3Backend) Initialize(ctx context.Context, config BackendConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse S3 configuration from connection string
	// Format: s3://bucket@region/path?endpoint=custom&ssl=true
	parts := strings.Split(config.ConnectionString, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid S3 connection string format")
	}

	s.bucket = strings.TrimPrefix(parts[0], "s3://")
	s.region = strings.Split(parts[1], "/")[0]
	s.accessKey = config.Username
	s.secretKey = config.Password

	// Parse additional options
	if params := config.Options; params != nil {
		if endpoint, ok := params["endpoint"].(string); ok {
			s.endpoint = endpoint
		}
		if useSSL, ok := params["ssl"].(bool); ok {
			s.useSSL = useSSL
		}
		if pathStyle, ok := params["path_style"].(bool); ok {
			s.pathStyle = pathStyle
		}
	}

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(s.region),
		Credentials:      credentials.NewStaticCredentials(s.accessKey, s.secretKey, ""),
		Endpoint:         aws.String(s.endpoint),
		DisableSSL:       aws.Bool(!s.useSSL),  
		S3ForcePathStyle: aws.Bool(s.pathStyle),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Test connection by listing bucket
	svc := s3.New(sess)
	_, err = svc.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	
	return err
}

func (s *S3Backend) Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error) {
	sess, err := s.getSession()
	if err != nil {
		return nil, nil, err
	}

	svc := s3.New(sess)
	
	result, err := svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, nil, err
	}

	metadata := &StorageMetadata{
		Key:         key,
		Size:        *result.ContentLength,
		ContentType: *result.ContentType,
		Checksum:    strings.Trim(*result.ETag, "\""),
		ModifiedAt:  *result.LastModified,
	}

	if result.ContentEncoding != nil {
		metadata.Encoding = *result.ContentEncoding
		metadata.Compressed = *result.ContentEncoding != ""
	}

	return data, metadata, nil
}

func (s *S3Backend) Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error {
	sess, err := s.getSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	if metadata != nil {
		if metadata.ContentType != "" {
			input.ContentType = aws.String(metadata.ContentType)
		}
		if metadata.Encoding != "" {
			input.ContentEncoding = aws.String(metadata.Encoding)
		}
		
		// Add custom metadata
		input.Metadata = map[string]*string{
			"cache-version": aws.String(strconv.FormatInt(metadata.Version, 10)),
			"created-at":    aws.String(metadata.CreatedAt.Format(time.RFC3339)),
		}
	}

	_, err = svc.PutObjectWithContext(ctx, input)
	return err
}

func (s *S3Backend) Delete(ctx context.Context, key string) error {
	sess, err := s.getSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	
	_, err = svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	return err
}

func (s *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	sess, err := s.getSession()
	if err != nil {
		return false, err
	}

	svc := s3.New(sess)
	
	_, err = svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return false, nil // Assume not found
	}
	
	return true, nil
}

func (s *S3Backend) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	// S3 doesn't support batch operations natively, so we'll do individual gets
	result := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Limit concurrent operations
	semaphore := make(chan struct{}, 10)
	
	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			data, _, err := s.Get(ctx, k)
			if err == nil {
				mu.Lock()
				result[k] = data
				mu.Unlock()
			}
		}(key)
	}
	
	wg.Wait()
	return result, nil
}

func (s *S3Backend) PutBatch(ctx context.Context, entries map[string][]byte) error {
	// S3 doesn't support batch operations natively, so we'll do individual puts
	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex
	
	// Limit concurrent operations
	semaphore := make(chan struct{}, 10)
	
	for key, data := range entries {
		wg.Add(1)
		go func(k string, d []byte) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := s.Put(ctx, k, d, nil); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(key, data)
	}
	
	wg.Wait()
	
	if len(errors) > 0 {
		return fmt.Errorf("batch put failed with %d errors: %v", len(errors), errors[0])
	}
	
	return nil
}

func (s *S3Backend) DeleteBatch(ctx context.Context, keys []string) error {
	sess, err := s.getSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	
	// S3 supports batch delete up to 1000 objects
	const maxBatchSize = 1000
	
	for i := 0; i < len(keys); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		
		batch := keys[i:end]
		objects := make([]*s3.ObjectIdentifier, len(batch))
		
		for j, key := range batch {
			objects[j] = &s3.ObjectIdentifier{
				Key: aws.String(key),
			}
		}
		
		_, err = svc.DeleteObjectsWithContext(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &s3.Delete{
				Objects: objects,
			},
		})
		
		if err != nil {
			return err
		}
	}
	
	return nil
}

func (s *S3Backend) Ping(ctx context.Context) error {
	sess, err := s.getSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	
	_, err = svc.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	
	return err
}

func (s *S3Backend) GetStats(ctx context.Context) (*BackendStats, error) {
	start := time.Now()
	err := s.Ping(ctx)
	responseTime := time.Since(start)
	
	return &BackendStats{
		Endpoint:     s.endpoint,
		Healthy:      err == nil,
		ResponseTime: responseTime,
		LastCheck:    time.Now(),
	}, nil
}

func (s *S3Backend) Close() error {
	// Nothing to close for S3 client
	return nil
}

func (s *S3Backend) getSession() (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region:           aws.String(s.region),
		Credentials:      credentials.NewStaticCredentials(s.accessKey, s.secretKey, ""),
		Endpoint:         aws.String(s.endpoint),
		DisableSSL:       aws.Bool(!s.useSSL),
		S3ForcePathStyle: aws.Bool(s.pathStyle),
	})
}

// RedisBackend implementation

func (r *RedisBackend) GetBackendType() BackendType {
	return BackendRedis
}

func (r *RedisBackend) GetEndpoint() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client.Options().Addrs[0]
}

func (r *RedisBackend) Initialize(ctx context.Context, config BackendConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Parse Redis configuration
	addrs := strings.Split(config.ConnectionString, ",")
	
	r.client = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        addrs,
		Password:     config.Password,
		DialTimeout:  time.Duration(config.TimeoutMs) * time.Millisecond,
		ReadTimeout:  time.Duration(config.TimeoutMs) * time.Millisecond,
		WriteTimeout: time.Duration(config.TimeoutMs) * time.Millisecond,
		PoolSize:     config.MaxConnections,
	})

	r.timeout = time.Duration(config.TimeoutMs) * time.Millisecond
	
	if keyPrefix, ok := config.Options["key_prefix"].(string); ok {
		r.keyPrefix = keyPrefix
	}
	
	if db, ok := config.Options["db"].(int); ok {
		r.db = db
	}
	
	if compression, ok := config.Options["compression"].(bool); ok {
		r.compression = compression
	}

	// Test connection
	return r.Ping(ctx)
}

func (r *RedisBackend) Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error) {
	fullKey := r.getFullKey(key)
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Get data and metadata in a pipeline for efficiency
	pipe := r.client.Pipeline()
	dataCmd := pipe.Get(ctx, fullKey)
	metaCmd := pipe.HGetAll(ctx, fullKey+":meta")
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		if err == redis.Nil {
			return nil, nil, ErrCacheEntryNotFound
		}
		return nil, nil, err
	}

	data, err := dataCmd.Bytes()
	if err != nil {
		return nil, nil, err
	}

	metadata := &StorageMetadata{
		Key:  key,
		Size: int64(len(data)),
	}

	// Parse metadata
	metaData := metaCmd.Val()
	if contentType, ok := metaData["content_type"]; ok {
		metadata.ContentType = contentType
	}
	if encoding, ok := metaData["encoding"]; ok {
		metadata.Encoding = encoding
		metadata.Compressed = encoding != ""
	}
	if checksum, ok := metaData["checksum"]; ok {
		metadata.Checksum = checksum
	}
	if version, ok := metaData["version"]; ok {
		if v, err := strconv.ParseInt(version, 10, 64); err == nil {
			metadata.Version = v
		}
	}
	if createdAt, ok := metaData["created_at"]; ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			metadata.CreatedAt = t
		}
	}

	return data, metadata, nil
}

func (r *RedisBackend) Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error {
	fullKey := r.getFullKey(key)
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Store data and metadata in a pipeline
	pipe := r.client.Pipeline()
	
	// Store the actual data
	pipe.Set(ctx, fullKey, data, 0) // 0 means no expiration
	
	// Store metadata
	if metadata != nil {
		metaData := map[string]interface{}{
			"content_type": metadata.ContentType,
			"size":         metadata.Size,
			"version":      metadata.Version,
			"created_at":   metadata.CreatedAt.Format(time.RFC3339),
			"modified_at":  time.Now().Format(time.RFC3339),
		}
		
		if metadata.Encoding != "" {
			metaData["encoding"] = metadata.Encoding
		}
		if metadata.Checksum != "" {
			metaData["checksum"] = metadata.Checksum
		}
		
		pipe.HMSet(ctx, fullKey+":meta", metaData)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBackend) Delete(ctx context.Context, key string) error {
	fullKey := r.getFullKey(key)
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Delete both data and metadata
	pipe := r.client.Pipeline()
	pipe.Del(ctx, fullKey)
	pipe.Del(ctx, fullKey+":meta")
	
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBackend) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := r.getFullKey(key)
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	result, err := r.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}
	
	return result == 1, nil
}

func (r *RedisBackend) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Prepare full keys
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = r.getFullKey(key)
	}

	// Use MGET for efficient batch retrieval
	values, err := r.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, value := range values {
		if value != nil {
			if str, ok := value.(string); ok {
				result[keys[i]] = []byte(str)
			}
		}
	}

	return result, nil
}

func (r *RedisBackend) PutBatch(ctx context.Context, entries map[string][]byte) error {
	if len(entries) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Use pipeline for batch operations
	pipe := r.client.Pipeline()

	for key, data := range entries {
		fullKey := r.getFullKey(key)
		pipe.Set(ctx, fullKey, data, 0)
		
		// Add basic metadata
		metaData := map[string]interface{}{
			"size":        len(data),
			"created_at":  time.Now().Format(time.RFC3339),
			"modified_at": time.Now().Format(time.RFC3339),
		}
		pipe.HMSet(ctx, fullKey+":meta", metaData)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBackend) DeleteBatch(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Prepare keys for deletion (both data and metadata)
	var keysToDelete []string
	for _, key := range keys {
		fullKey := r.getFullKey(key)
		keysToDelete = append(keysToDelete, fullKey, fullKey+":meta")
	}

	return r.client.Del(ctx, keysToDelete...).Err()
}

func (r *RedisBackend) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	
	return r.client.Ping(ctx).Err()
}

func (r *RedisBackend) GetStats(ctx context.Context) (*BackendStats, error) {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	
	// Get Redis info
	info, err := r.client.Info(ctx, "stats").Result()
	responseTime := time.Since(start)
	
	stats := &BackendStats{
		Endpoint:     r.GetEndpoint(),
		Healthy:      err == nil,
		ResponseTime: responseTime,
		LastCheck:    time.Now(),
	}

	if err == nil {
		// Parse Redis stats (simplified)
		lines := strings.Split(info, "\r\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "total_commands_processed:") {
				if value := strings.Split(line, ":")[1]; value != "" {
					if count, err := strconv.ParseInt(value, 10, 64); err == nil {
						stats.RequestCount = count
					}
				}
			}
		}
	}

	return stats, nil
}

func (r *RedisBackend) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.client != nil {
		return r.client.Close()
	}
	
	return nil
}

func (r *RedisBackend) getFullKey(key string) string {
	if r.keyPrefix != "" {
		return r.keyPrefix + ":" + key
	}
	return key
}

// CustomBackend implementation

func (c *CustomBackend) GetBackendType() BackendType {
	return BackendCustom
}

func (c *CustomBackend) GetEndpoint() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.endpoint
}

func (c *CustomBackend) Initialize(ctx context.Context, config BackendConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.endpoint = config.ConnectionString
	c.config = config.Options

	if protocol, ok := config.Options["protocol"].(string); ok {
		c.protocol = protocol
	}

	// Initialize custom handler based on protocol
	if c.handler != nil {
		return c.handler.Configure(c.config)
	}

	return nil
}

func (c *CustomBackend) Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error) {
	if c.handler == nil {
		return nil, nil, errors.New("custom handler not configured")
	}

	data, err := c.handler.Execute(ctx, fmt.Sprintf("GET:%s", key), nil)
	if err != nil {
		return nil, nil, err
	}

	// Parse response (format depends on custom protocol)
	metadata := &StorageMetadata{
		Key:  key,
		Size: int64(len(data)),
	}

	return data, metadata, nil
}

func (c *CustomBackend) Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error {
	if c.handler == nil {
		return errors.New("custom handler not configured")
	}

	_, err := c.handler.Execute(ctx, fmt.Sprintf("PUT:%s", key), data)
	return err
}

func (c *CustomBackend) Delete(ctx context.Context, key string) error {
	if c.handler == nil {
		return errors.New("custom handler not configured")
	}

	_, err := c.handler.Execute(ctx, fmt.Sprintf("DELETE:%s", key), nil)
	return err
}

func (c *CustomBackend) Exists(ctx context.Context, key string) (bool, error) {
	if c.handler == nil {
		return false, errors.New("custom handler not configured")
	}

	result, err := c.handler.Execute(ctx, fmt.Sprintf("EXISTS:%s", key), nil)
	if err != nil {
		return false, err
	}

	return string(result) == "true", nil
}

func (c *CustomBackend) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	if c.handler == nil {
		return nil, errors.New("custom handler not configured")
	}

	// Serialize keys for batch operation
	keysData, _ := json.Marshal(keys)
	result, err := c.handler.Execute(ctx, "GET_BATCH", keysData)
	if err != nil {
		return nil, err
	}

	// Deserialize response
	var batchResult map[string][]byte
	if err := json.Unmarshal(result, &batchResult); err != nil {
		return nil, err
	}

	return batchResult, nil
}

func (c *CustomBackend) PutBatch(ctx context.Context, entries map[string][]byte) error {
	if c.handler == nil {
		return errors.New("custom handler not configured")
	}

	// Serialize entries for batch operation
	entriesData, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	_, err = c.handler.Execute(ctx, "PUT_BATCH", entriesData)
	return err
}

func (c *CustomBackend) DeleteBatch(ctx context.Context, keys []string) error {
	if c.handler == nil {
		return errors.New("custom handler not configured")
	}

	// Serialize keys for batch operation
	keysData, _ := json.Marshal(keys)
	_, err := c.handler.Execute(ctx, "DELETE_BATCH", keysData)
	return err
}

func (c *CustomBackend) Ping(ctx context.Context) error {
	if c.handler == nil {
		return errors.New("custom handler not configured")
	}

	return c.handler.HealthCheck(ctx)
}

func (c *CustomBackend) GetStats(ctx context.Context) (*BackendStats, error) {
	start := time.Now()
	err := c.Ping(ctx)
	responseTime := time.Since(start)

	return &BackendStats{
		Endpoint:     c.endpoint,
		Healthy:      err == nil,
		ResponseTime: responseTime,
		LastCheck:    time.Now(),
	}, nil
}

func (c *CustomBackend) Close() error {
	// Custom handler cleanup would go here
	return nil
}

// Utility functions

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// Error definitions
var (
	ErrCacheEntryNotFound = errors.New("cache entry not found")
)