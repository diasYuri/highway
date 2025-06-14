# Highway 🛣️

A high-performance, type-safe streaming library for Go with built-in parallel processing capabilities.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen)](https://github.com/diasYuri/highway/actions)

## Overview

Highway is a modern Go library that provides a fluent API for processing data streams with built-in support for:

- **Type Safety**: Full generic type support with compile-time safety
- **Parallel Processing**: Built-in worker pool for concurrent operations
- **Memory Efficient**: Optimized for low memory footprint and high throughput
- **Time Windows**: Group data by time intervals for batch processing
- **Functional Style**: Chainable operations with map, filter, and reduce patterns

## Installation

```bash
go get github.com/diasYuri/highway
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/diasYuri/highway/pkg"
)

func main() {
    // Create a stream from a slice
    data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
    
    // Process the stream
    err := pkg.NewSliceStream(data).
        Map(func(x int) int { return x * 2 }).        // Double each number
        Where(func(x int) bool { return x > 10 }).    // Filter > 10
        ForEach(func(x int) { fmt.Println(x) }).      // Print each
        Sink(func(x int) { /* final processing */ })
    
    if err != nil {
        panic(err)
    }
    // Output: 12, 14, 16, 18, 20
}
```

### Parallel Processing

```go
package main

import (
    "fmt"
    "runtime"
    "github.com/diasYuri/highway/pkg"
)

func main() {
    data := make([]int, 10000)
    for i := range data {
        data[i] = i + 1
    }
    
    // Process with parallel workers
    err := pkg.NewSliceStream(data).
        Parallel(runtime.NumCPU()).               // Use all CPU cores
        Map(func(x int) int { return x * x }).    // Square each number
        Where(func(x int) bool { return x%2 == 0 }). // Even squares only
        Sink(func(x int) {
            fmt.Printf("Even square: %d\n", x)
        })
    
    if err != nil {
        panic(err)
    }
}
```

### Channel Streams

```go
package main

import (
    "fmt"
    "time"
    "github.com/diasYuri/highway/pkg"
)

func main() {
    // Create a channel
    ch := make(chan int, 10)
    
    // Producer goroutine
    go func() {
        defer close(ch)
        for i := 1; i <= 100; i++ {
            ch <- i
            time.Sleep(10 * time.Millisecond)
        }
    }()
    
    // Process channel stream
    err := pkg.NewChanStream(ch).
        Map(func(x int) int { return x * 10 }).
        Sink(func(x int) {
            fmt.Printf("Processed: %d\n", x)
        })
    
    if err != nil {
        panic(err)
    }
}
```

### Time Window Batching

```go
package main

import (
    "fmt"
    "time"
    "github.com/diasYuri/highway/pkg"
)

func main() {
    ch := make(chan int, 100)
    
    // Simulate real-time data
    go func() {
        defer close(ch)
        for i := 1; i <= 50; i++ {
            ch <- i
            time.Sleep(50 * time.Millisecond)
        }
    }()
    
    // Group by time windows
    err := pkg.NewChanStream(ch).
        GroupByTimeWindow(200 * time.Millisecond).
        Sink(func(batch []int) {
            fmt.Printf("Batch of %d items: %v\n", len(batch), batch)
        })
    
    if err != nil {
        panic(err)
    }
}
```

## API Reference

### Stream Creation

#### `NewSliceStream[T any](data []T) Stream[T, T]`
Creates a stream from a slice of data.

#### `NewChanStream[T any](ch <-chan T) Stream[T, T]`
Creates a stream from a Go channel.

### Stream Operations

#### `Map(fn func(T) R) Stream[R, R]`
Transforms each element using the provided function.

```go
stream.Map(func(x int) string {
    return fmt.Sprintf("item_%d", x)
})
```

#### `Where(fn func(T) bool) Stream[T, R]`
Filters elements based on a predicate function.

```go
stream.Where(func(x int) bool {
    return x%2 == 0  // Keep only even numbers
})
```

#### `ForEach(fn func(T)) Stream[T, R]`
Applies a function to each element (for side effects) and passes the element through.

```go
stream.ForEach(func(x int) {
    log.Printf("Processing: %d", x)
})
```

#### `Parallel(workers int) Stream[T, R]`
Configures the stream to use parallel processing with the specified number of workers.

```go
stream.Parallel(4)  // Use 4 worker goroutines
```

#### `GroupByTimeWindow(timeWindow time.Duration) StreamBatch[T, R]`
Groups elements into batches based on time intervals.

```go
stream.GroupByTimeWindow(100 * time.Millisecond)
```

#### `Sink(fn func(T)) error`
Terminal operation that consumes all elements from the stream.

```go
err := stream.Sink(func(x int) {
    // Final processing
    database.Save(x)
})
```

### Batch Operations

#### `StreamBatch.Sink(fn func([]T)) error`
Processes batches of elements from time-windowed streams.

```go
err := stream.GroupByTimeWindow(duration).Sink(func(batch []int) {
    // Process entire batch
    fmt.Printf("Batch size: %d\n", len(batch))
})
```

## Performance Benchmarks

Based on Apple M1 Pro with 1000 elements:

| Operation | Time/op | Memory/op | Allocs/op |
|-----------|---------|-----------|-----------|
| SliceStream | 40.3µs | 8.3KB | 5 |
| MapOperation | 176.1µs | 8.5KB | 9 |
| ParallelMap | 241.1µs | 9.3KB | 27 |
| WhereOperation | 117.7µs | 8.5KB | 9 |
| ChainedOperations | 514.2µs | 8.8KB | 17 |
| ParallelChained | 588.0µs | 10.7KB | 53 |
| GroupByTimeWindow | 30.0µs | 3.6KB | 17 |

*Benchmarks run with `go test -bench=. -benchmem`*

## Advanced Examples

### Complex Data Processing Pipeline

```go
type User struct {
    ID   int
    Name string
    Age  int
}

type ProcessedUser struct {
    ID   int
    Name string
    Category string
}

users := []User{
    {1, "Alice", 25},
    {2, "Bob", 30},
    {3, "Charlie", 35},
    // ... more users
}

err := pkg.NewSliceStream(users).
    Parallel(4).
    Where(func(u User) bool { return u.Age >= 18 }). // Adults only
    Map(func(u User) ProcessedUser {
        category := "young"
        if u.Age >= 30 {
            category = "mature"
        }
        return ProcessedUser{u.ID, u.Name, category}
    }).
    ForEach(func(pu ProcessedUser) {
        // Log processing
        log.Printf("Processed user %s as %s", pu.Name, pu.Category)
    }).
    Sink(func(pu ProcessedUser) {
        // Save to database
        database.SaveProcessedUser(pu)
    })
```

### Real-time Event Processing

```go
type Event struct {
    Timestamp time.Time
    Type      string
    Data      interface{}
}

eventChannel := make(chan Event, 1000)

// Start event processor
go func() {
    err := pkg.NewChanStream(eventChannel).
        Where(func(e Event) bool {
            return e.Type == "user_action"
        }).
        GroupByTimeWindow(5 * time.Second).
        Sink(func(events []Event) {
            // Process batch of events
            analytics.ProcessBatch(events)
            fmt.Printf("Processed batch of %d events\n", len(events))
        })
    
    if err != nil {
        log.Printf("Error processing events: %v", err)
    }
}()

// Send events
for i := 0; i < 1000; i++ {
    eventChannel <- Event{
        Timestamp: time.Now(),
        Type:      "user_action",
        Data:      fmt.Sprintf("action_%d", i),
    }
    time.Sleep(100 * time.Millisecond)
}
close(eventChannel)
```

## Testing

The library includes comprehensive tests covering:

- **Unit Tests**: All core functionality
- **Concurrency Tests**: Race condition detection
- **Performance Tests**: Benchmarking and memory usage
- **Edge Cases**: Error handling and boundary conditions
- **Stress Tests**: High concurrency scenarios

Run tests:

```bash
# Run all tests
go test ./pkg -v

# Run tests with race detection
go test ./pkg -race -v

# Run benchmarks
go test ./pkg -bench=. -benchmem

# Run performance tests
go test ./pkg -v -run="TestPerformance"
```

## Best Practices

### 1. Choose Appropriate Worker Count
```go
// Good: Use runtime.NumCPU() for CPU-bound tasks
stream.Parallel(runtime.NumCPU())

// Good: Use higher count for I/O-bound tasks
stream.Parallel(runtime.NumCPU() * 2)

// Avoid: Too many workers can hurt performance
stream.Parallel(1000) // Usually not beneficial
```

### 2. Handle Errors Properly
```go
err := stream.Sink(func(item int) {
    if err := processItem(item); err != nil {
        log.Printf("Error processing item %d: %v", item, err)
    }
})

if err != nil {
    return fmt.Errorf("stream processing failed: %w", err)
}
```

### 3. Use Appropriate Time Windows
```go
// Good: Reasonable time window for batching
stream.GroupByTimeWindow(100 * time.Millisecond)

// Avoid: Too small windows create overhead
stream.GroupByTimeWindow(1 * time.Nanosecond)

// Avoid: Too large windows increase latency
stream.GroupByTimeWindow(1 * time.Hour)
```

### 4. Memory Management
```go
// Good: Process data in chunks for large datasets
const chunkSize = 1000
for i := 0; i < len(largeData); i += chunkSize {
    end := min(i+chunkSize, len(largeData))
    chunk := largeData[i:end]
    
    err := pkg.NewSliceStream(chunk).
        Parallel(4).
        // ... process chunk
        Sink(processor)
}
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Write tests for new features
- Run `go test -race` to check for race conditions
- Follow Go naming conventions
- Add benchmarks for performance-critical code
- Update documentation for API changes

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📧 Email: [your-email@example.com](mailto:your-email@example.com)
- 🐛 Issues: [GitHub Issues](https://github.com/diasYuri/highway/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/diasYuri/highway/discussions)

## Acknowledgments

- Inspired by Java Streams and RxJava
- Built with Go's powerful concurrency primitives
- Designed for high-performance data processing workflows

---

Made with ❤️ by [Yuri Dias](https://github.com/diasYuri) 