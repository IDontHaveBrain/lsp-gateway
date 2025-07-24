package installer

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadOptions contains options for file downloads
type DownloadOptions struct {
	URL              string
	OutputPath       string
	ExpectedChecksum string
	Timeout          time.Duration
	MaxRetries       int
	ProgressCallback func(downloaded, total int64)
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Success          bool
	FilePath         string
	ActualChecksum   string
	ExpectedChecksum string
	FileSize         int64
	Duration         time.Duration
	Verified         bool
	Error            error
}

// FileDownloader handles secure file downloads with checksum verification
type FileDownloader struct {
	client *http.Client
}

// NewFileDownloader creates a new file downloader with reasonable defaults
func NewFileDownloader() *FileDownloader {
	return &FileDownloader{
		client: &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				ExpectContinueTimeout: 10 * time.Second,
			},
		},
	}
}

// Download downloads a file from the specified URL with checksum verification
func (d *FileDownloader) Download(options DownloadOptions) *DownloadResult {
	start := time.Now()

	result := &DownloadResult{
		ExpectedChecksum: options.ExpectedChecksum,
	}

	// Set default timeout if not specified
	if options.Timeout == 0 {
		options.Timeout = 10 * time.Minute
	}

	// Set default max retries if not specified
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}

	// Create a client with the specified timeout
	client := &http.Client{
		Timeout: options.Timeout,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
		},
	}

	var lastErr error
	for attempt := 1; attempt <= options.MaxRetries; attempt++ {
		if attempt > 1 {
			// Exponential backoff for retries
			waitTime := time.Duration(attempt*attempt) * time.Second
			time.Sleep(waitTime)
		}

		err := d.downloadAttempt(client, options, result)
		if err == nil {
			result.Success = true
			result.Duration = time.Since(start)
			return result
		}

		lastErr = err

		// Clean up partial download on failure
		if result.FilePath != "" {
			_ = os.Remove(result.FilePath)
		}
	}

	result.Error = fmt.Errorf("download failed after %d attempts: %w", options.MaxRetries, lastErr)
	result.Duration = time.Since(start)
	return result
}

// downloadAttempt performs a single download attempt
func (d *FileDownloader) downloadAttempt(client *http.Client, options DownloadOptions, result *DownloadResult) error {
	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(options.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the output file
	outFile, err := os.Create(options.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Make the HTTP request
	resp, err := client.Get(options.URL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Get content length for progress tracking
	contentLength := resp.ContentLength

	// Create a hash writer to calculate checksum while downloading
	hasher := sha256.New()
	var writer io.Writer = outFile

	// Add hasher to the writer chain
	writer = io.MultiWriter(writer, hasher)

	// Add progress tracking if callback is provided
	if options.ProgressCallback != nil {
		progressReader := &progressReader{
			reader:   resp.Body,
			total:    contentLength,
			callback: options.ProgressCallback,
		}
		resp.Body = progressReader
	}

	// Copy the response body to the file while calculating checksum
	written, err := io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate the actual checksum
	actualChecksum := fmt.Sprintf("%x", hasher.Sum(nil))

	// Update result
	result.FilePath = options.OutputPath
	result.ActualChecksum = actualChecksum
	result.FileSize = written

	// Verify checksum if expected checksum is provided
	if options.ExpectedChecksum != "" {
		if strings.EqualFold(actualChecksum, options.ExpectedChecksum) {
			result.Verified = true
		} else {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", options.ExpectedChecksum, actualChecksum)
		}
	} else {
		// If no expected checksum, mark as verified (download was successful)
		result.Verified = true
	}

	return nil
}

// ExtractTarGz extracts a .tar.gz file to the specified directory
func ExtractTarGz(archivePath, destDir string) error {
	// Open the archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Construct the full path
		destPath := filepath.Join(destDir, header.Name)

		// Security check: ensure path is within destination directory
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}

		case tar.TypeReg:
			// Create parent directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
			}

			// Create and write file
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
			_ = outFile.Close()

		case tar.TypeSymlink:
			// Create symlink
			if err := os.Symlink(header.Linkname, destPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", destPath, err)
			}
		}
	}

	return nil
}

// progressReader wraps an io.ReadCloser to provide progress callbacks
type progressReader struct {
	reader     io.ReadCloser
	total      int64
	downloaded int64
	callback   func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	if pr.callback != nil {
		pr.callback(pr.downloaded, pr.total)
	}

	return n, err
}

func (pr *progressReader) Close() error {
	return pr.reader.Close()
}

// VerifyFileChecksum verifies the SHA256 checksum of a file
func VerifyFileChecksum(filePath, expectedChecksum string) (bool, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false, "", fmt.Errorf("failed to read file: %w", err)
	}

	actualChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	isValid := strings.EqualFold(actualChecksum, expectedChecksum)

	return isValid, actualChecksum, nil
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return info.Size(), nil
}
