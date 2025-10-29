package cache

import (
	"sync"
)

// WorkerPool provides a generic worker pool for parallel processing of items.
// It manages goroutines, channels, and synchronization internally.
type WorkerPool struct {
	workerCount int
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
func NewWorkerPool(workerCount int) *WorkerPool {
	if workerCount < 1 {
		workerCount = 1
	}
	return &WorkerPool{workerCount: workerCount}
}

// Execute processes items in parallel using a worker pool.
// The executor function is called for each item index.
// Returns any errors encountered during processing.
func (p *WorkerPool) Execute(itemCount int, executor func(index int) error) []error {
	if itemCount == 0 {
		return nil
	}

	jobs := make(chan int, p.workerCount)
	errorCh := make(chan error, itemCount)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if err := executor(idx); err != nil {
					errorCh <- err
				}
			}
		}()
	}

	// Send work to workers
	for i := 0; i < itemCount; i++ {
		jobs <- i
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(errorCh)

	// Collect errors
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	return errors
}

// ExecuteWithProgress processes items in parallel with progress tracking.
// The progress callback is called after each item completes (thread-safe).
func (p *WorkerPool) ExecuteWithProgress(
	itemCount int,
	executor func(index int) error,
	progress func(completed, total int),
) []error {
	if itemCount == 0 {
		return nil
	}

	jobs := make(chan int, p.workerCount)
	errorCh := make(chan error, itemCount)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	completed := 0

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				err := executor(idx)
				if err != nil {
					errorCh <- err
				}

				// Update progress
				if progress != nil {
					progressMu.Lock()
					completed++
					currentCompleted := completed
					progressMu.Unlock()
					progress(currentCompleted, itemCount)
				}
			}
		}()
	}

	// Send work to workers
	for i := 0; i < itemCount; i++ {
		jobs <- i
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(errorCh)

	// Collect errors
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	return errors
}

// ExecuteWithCustomProgress processes items with a custom progress callback.
// The progressCallback receives the item index and total count for each completion.
// This variant allows more flexible progress tracking (e.g., with labels).
func (p *WorkerPool) ExecuteWithCustomProgress(
	itemCount int,
	executor func(index int) error,
	progressCallback func(index, total int, label string),
	label string,
) []error {
	if itemCount == 0 {
		return nil
	}

	jobs := make(chan int, p.workerCount)
	errorCh := make(chan error, itemCount)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				err := executor(idx)
				if err != nil {
					errorCh <- err
				}

				// Call progress callback
				if progressCallback != nil {
					progressCallback(idx+1, itemCount, label)
				}
			}
		}()
	}

	// Send work to workers
	for i := 0; i < itemCount; i++ {
		jobs <- i
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(errorCh)

	// Collect errors
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	return errors
}
