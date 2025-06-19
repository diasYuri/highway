package pkg

import (
	"context"
	"sync"
	"time"
)

type stream[T any, R any] struct {
	source  <-chan T
	workers int
}

type streamBatch[T any, R any] struct {
	source  <-chan []T
	workers int
}

const (
	// Maximum buffer size to prevent memory explosion
	maxBufferSize = 1024
)

func NewSliceStream[T any](data []T) Stream[T, T] {
	// Strategic buffering: use smaller buffer to prevent memory explosion
	bufferSize := len(data)
	if bufferSize > maxBufferSize {
		bufferSize = maxBufferSize
	}

	source := make(chan T, bufferSize)
	go func() {
		defer close(source)
		for _, item := range data {
			source <- item
		}
	}()
	return &stream[T, T]{source: source, workers: 1}
}

func NewChanStream[T any](ch <-chan T) Stream[T, T] {
	// Eliminate unnecessary copy by using the original channel directly
	return &stream[T, T]{source: ch, workers: 1}
}

func newStream[T any, R any](source chan T, workers int) Stream[T, R] {
	return &stream[T, R]{source: source, workers: workers}
}

func (s *stream[T, R]) Map(fn func(T) R) Stream[R, R] {
	out := make(chan R, s.workers)

	go func(channel chan<- R) {
		defer close(channel)

		if s.workers == 1 {
			for item := range s.source {
				channel <- fn(item)
			}
			return
		}

		// Fix race condition: use proper fan-out pattern
		var wg sync.WaitGroup

		// Create a buffered channel to distribute work
		workChan := make(chan T, s.workers)

		// Start workers
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range workChan {
					channel <- fn(item)
				}
			}()
		}

		// Distribute work
		go func() {
			defer close(workChan)
			for item := range s.source {
				workChan <- item
			}
		}()

		wg.Wait()
	}(out)

	return newStream[R, R](out, s.workers)
}

func (s *stream[T, R]) Where(fn func(T) bool) Stream[T, R] {
	out := make(chan T, s.workers)

	go func(channel chan<- T) {
		defer close(channel)

		if s.workers == 1 {
			for item := range s.source {
				if fn(item) {
					channel <- item
				}
			}
			return
		}

		// Fix race condition: use proper fan-out pattern
		var wg sync.WaitGroup

		// Create a buffered channel to distribute work
		workChan := make(chan T, s.workers)

		// Start workers
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range workChan {
					if fn(item) {
						channel <- item
					}
				}
			}()
		}

		// Distribute work
		go func() {
			defer close(workChan)
			for item := range s.source {
				workChan <- item
			}
		}()

		wg.Wait()
	}(out)

	return newStream[T, R](out, s.workers)
}

func (s *stream[T, R]) ForEach(fn func(T)) Stream[T, R] {
	out := make(chan T, s.workers)

	go func(channel chan<- T) {
		defer close(channel)

		if s.workers == 1 {
			for item := range s.source {
				fn(item)
				channel <- item
			}
			return
		}

		// Fix race condition: use proper fan-out pattern
		var wg sync.WaitGroup

		// Create a buffered channel to distribute work
		workChan := make(chan T, s.workers)

		// Start workers
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range workChan {
					fn(item)
					channel <- item
				}
			}()
		}

		// Distribute work
		go func() {
			defer close(workChan)
			for item := range s.source {
				workChan <- item
			}
		}()

		wg.Wait()
	}(out)

	return newStream[T, R](out, s.workers)
}

func (s *stream[T, R]) Parallel(workers int) Stream[T, R] {
	if workers <= 0 {
		workers = 1
	}
	// Make immutable: create new stream instead of modifying existing one
	return &stream[T, R]{source: s.source, workers: workers}
}

func (s *stream[T, R]) Sink(fn func(T)) error {
	if s.workers == 1 {
		for item := range s.source {
			fn(item)
		}
		return nil
	}

	// Fix race condition: use proper fan-out pattern
	var wg sync.WaitGroup

	// Create a buffered channel to distribute work
	workChan := make(chan T, s.workers)

	// Start workers
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workChan {
				fn(item)
			}
		}()
	}

	// Distribute work
	go func() {
		defer close(workChan)
		for item := range s.source {
			workChan <- item
		}
	}()

	wg.Wait()
	return nil
}

func (s *stream[T, R]) Collect(ctx context.Context) ([]T, error) {
	var result []T

	for {
		select {
		case item, ok := <-s.source:
			if !ok {
				return result, nil
			}
			result = append(result, item)
		case <-ctx.Done():
			return nil, ctx.Err()
			// Removed default case to fix busy loop - this was causing 100% CPU usage
		}
	}
}

func (s *stream[T, R]) GroupByTimeWindow(timeWindow time.Duration) StreamBatch[T, R] {
	out := make(chan []T, s.workers)

	go func(channel chan<- []T) {
		defer close(channel)

		currentBatch := make([]T, 0, 64)
		var ticker *time.Ticker
		var tickerCh <-chan time.Time

		for {
			select {
			case item, ok := <-s.source:
				if !ok {
					if len(currentBatch) > 0 {
						// Optimize: avoid unnecessary copy by reusing the slice
						channel <- currentBatch
					}
					if ticker != nil {
						ticker.Stop()
					}
					return
				}

				if len(currentBatch) == 0 {
					if ticker != nil {
						ticker.Stop()
					}
					ticker = time.NewTicker(timeWindow)
					tickerCh = ticker.C
				}

				currentBatch = append(currentBatch, item)

			case <-tickerCh:
				if len(currentBatch) > 0 {
					// Send current batch and create new one
					channel <- currentBatch
					currentBatch = make([]T, 0, 64)
				}
				if ticker != nil {
					ticker.Stop()
					tickerCh = nil
				}
			}
		}
	}(out)

	return &streamBatch[T, R]{source: out, workers: s.workers}
}

func (s *streamBatch[T, R]) Sink(fn func([]T)) error {
	if s.workers == 1 {
		for item := range s.source {
			fn(item)
		}
		return nil
	}

	// Fix race condition: use proper fan-out pattern
	var wg sync.WaitGroup

	// Create a buffered channel to distribute work
	workChan := make(chan []T, s.workers)

	// Start workers
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workChan {
				fn(item)
			}
		}()
	}

	// Distribute work
	go func() {
		defer close(workChan)
		for item := range s.source {
			workChan <- item
		}
	}()

	wg.Wait()
	return nil
}

func (s *streamBatch[T, R]) Collect(ctx context.Context) ([][]T, error) {
	var result [][]T

	for {
		select {
		case item, ok := <-s.source:
			if !ok {
				return result, nil
			}
			result = append(result, item)
		case <-ctx.Done():
			return nil, ctx.Err()
			// Removed default case to fix busy loop
		}
	}
}
