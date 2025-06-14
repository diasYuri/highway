package pkg

import (
	"context"
	"time"
)

// Stream defines a generic interface for processing streams of data.
type Stream[T any, R any] interface {
	// Map transforms each element of the stream using the provided function
	Map(fn func(T) R) Stream[R, R]

	// Where filters the stream based on the provided predicate function
	Where(fn func(T) bool) Stream[T, R]

	// ForEach applies the provided function to each element of the stream
	ForEach(fn func(T)) Stream[T, R]

	// Parallel processes the stream in parallel using the specified number of workers
	Parallel(workers int) Stream[T, R]

	// GroupByTimeWindow stream events into batches based on a fixed time interval, emitting all events collected within each window together.
	GroupByTimeWindow(timeWindow time.Duration) StreamBatch[T, R]

	// Sink applies the provided function to each element of the stream, typically for side effects
	Sink(fn func(T)) error

	// Collect gathers all elements of the stream into a slice, returning it
	Collect(ctx context.Context) ([]T, error)
}

type StreamBatch[T any, R any] interface {
	// Sink applies the provided function to each element of the stream, typically for side effects
	Sink(fn func([]T)) error

	// Collect gathers all elements of the stream into a slice, returning it
	Collect(ctx context.Context) ([][]T, error)
}
