package pkg

import (
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

func NewSliceStream[T any](data []T) Stream[T, T] {
	source := make(chan T, len(data))
	go func() {
		defer close(source)
		for _, item := range data {
			source <- item
		}
	}()
	return &stream[T, T]{source: source, workers: 1}
}

func NewChanStream[T any](ch <-chan T) Stream[T, T] {
	source := make(chan T, 1)
	go func() {
		defer close(source)
		for item := range ch {
			source <- item
		}
	}()
	return &stream[T, T]{source: source, workers: 1}
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

		var wg sync.WaitGroup
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range s.source {
					channel <- fn(item)
				}
			}()
		}
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

		var wg sync.WaitGroup
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range s.source {
					if fn(item) {
						channel <- item
					}
				}
			}()
		}
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

		var wg sync.WaitGroup
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range s.source {
					fn(item)
					channel <- item
				}
			}()
		}
		wg.Wait()
	}(out)

	return newStream[T, R](out, s.workers)
}

func (s *stream[T, R]) Parallel(workers int) Stream[T, R] {
	if workers <= 0 {
		workers = 1
	}
	s.workers = workers
	return s
}

func (s *stream[T, R]) Sink(fn func(T)) error {
	if s.workers == 1 {
		for item := range s.source {
			fn(item)
		}
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range s.source {
				fn(item)
			}
		}()
	}
	wg.Wait()

	return nil
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
						finalBatch := make([]T, len(currentBatch))
						copy(finalBatch, currentBatch)
						channel <- finalBatch
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
					batch := make([]T, len(currentBatch))
					copy(batch, currentBatch)
					channel <- batch
					currentBatch = currentBatch[:0]
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

func (s streamBatch[T, R]) Sink(fn func([]T)) error {
	if s.workers == 1 {
		for item := range s.source {
			fn(item)
		}
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range s.source {
				fn(item)
			}
		}()
	}
	wg.Wait()

	return nil
}
