# Memory Leak Detection Guide

This guide explains how to use the memory leak detection feature in the pprof-analyzer-mcp tool to identify potential memory leaks in Go applications.

## Overview

The memory leak detection feature compares two heap profiles taken at different points in time and identifies memory allocations that have grown significantly. This helps you pinpoint potential memory leaks in your Go applications.

## How It Works

The memory leak detection algorithm:

1. Analyzes two heap profiles (an "old" profile and a "new" profile)
2. Identifies memory allocations by type that have grown beyond a specified threshold
3. Provides detailed information about memory growth, including:
   - Absolute memory growth (in bytes)
   - Percentage growth
   - Object count growth
   - Average object size

## Generating Heap Profiles

Before you can detect memory leaks, you need to generate heap profiles at different points in time. Here are several ways to do this:

### Method 1: Using `runtime/pprof` in Your Code

Add profiling code to your application:

```go
package main

import (
	"os"
	"runtime/pprof"
	// other imports
)

func main() {
	// Start your application
	
	// Generate first heap profile (before potential leak)
	f1, _ := os.Create("heap_before.pprof")
	pprof.WriteHeapProfile(f1)
	f1.Close()
	
	// Run your application for some time
	// ...
	
	// Generate second heap profile (after potential leak)
	f2, _ := os.Create("heap_after.pprof")
	pprof.WriteHeapProfile(f2)
	f2.Close()
}
```

### Method 2: Using HTTP pprof Endpoint

If your application uses the `net/http/pprof` package, you can fetch heap profiles via HTTP:

```bash
# First profile
curl -s http://localhost:6060/debug/pprof/heap > heap_before.pprof

# Wait some time...

# Second profile
curl -s http://localhost:6060/debug/pprof/heap > heap_after.pprof
```

### Method 3: Using `go tool pprof` for a Running Process

For a running Go process with PID `<pid>`:

```bash
# First profile
go tool pprof -inuse_space http://localhost:6060/debug/pprof/heap
# At the pprof prompt, type:
# > proto > heap_before.pprof
# > quit

# Wait some time...

# Second profile
go tool pprof -inuse_space http://localhost:6060/debug/pprof/heap
# At the pprof prompt, type:
# > proto > heap_after.pprof
# > quit
```

## Using the Memory Leak Detection Feature

### Via the MCP Server

The pprof-analyzer-mcp tool exposes a `detect_memory_leaks` tool that you can use via the MCP server:

```json
{
  "tool_name": "detect_memory_leaks",
  "arguments": {
    "old_profile_uri": "file:///path/to/your/heap_before.pprof",
    "new_profile_uri": "file:///path/to/your/heap_after.pprof",
    "threshold": 0.05,  // 5% growth threshold
    "limit": 15         // Show top 15 potential leaks
  }
}
```

### Parameters

- `old_profile_uri`: URI to the earlier heap profile (supports `file://`, `http://`, `https://`)
- `new_profile_uri`: URI to the later heap profile (supports `file://`, `http://`, `https://`)
- `threshold`: Growth threshold as a decimal (0.1 = 10%, 0.05 = 5%)
- `limit`: Maximum number of results to show

## Example: Detecting a Memory Leak

Here's a complete example of detecting a memory leak in a Go application:

1. Create a test program with a memory leak:

```go
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// Global variable to prevent garbage collection
var leakySlice []string

func main() {
	// Enable memory profiling
	runtime.MemProfileRate = 1

	// Create first heap profile (before leak)
	f1, _ := os.Create("heap_before.pprof")
	runtime.GC() // Force garbage collection
	pprof.WriteHeapProfile(f1)
	f1.Close()
	
	// Create some "leaky" allocations
	for i := 0; i < 10000; i++ {
		leakySlice = append(leakySlice, fmt.Sprintf("This is a string that will not be garbage collected: %d", i))
	}
	
	// Wait a moment to ensure memory operations complete
	time.Sleep(1 * time.Second)
	
	// Create second heap profile (after leak)
	f2, _ := os.Create("heap_after.pprof")
	runtime.GC() // Force garbage collection
	pprof.WriteHeapProfile(f2)
	f2.Close()
	
	fmt.Println("Heap profiles created: heap_before.pprof and heap_after.pprof")
}
```

2. Run the program to generate the profiles:

```bash
go run memory_leak_example.go
```

3. Use the memory leak detection tool:

```bash
# Using the MCP server (see above)

# Or using a command-line tool like the one in the examples:
go run test_memory_leak_detector.go -old=heap_before.pprof -new=heap_after.pprof -threshold=0.05 -limit=10
```

4. Analyze the results:

```
Memory Leak Detection Report
==========================

Found 1 types with significant memory growth (threshold: 5.0%)

Top Potential Memory Leaks:
--------------------------------------------------
Type                 Old Size        New Size        Growth          Growth %
--------------------------------------------------
string               17.47 KB        828.89 KB       811.42 KB       4644.99% (Objects: 45 → 10052, +10007, 22237.78%)

Recommendations:
1. Focus on types with both high absolute growth and high percentage growth
2. Look for objects that grow in count but not significantly in size (may indicate collection leaks)
3. Compare multiple snapshots over time to confirm consistent growth patterns
```

## Interpreting the Results

The memory leak detection report provides several key pieces of information:

1. **Type**: The type of object that's growing (e.g., string, slice, struct)
2. **Old Size**: Memory usage in the first profile
3. **New Size**: Memory usage in the second profile
4. **Growth**: Absolute memory growth (New Size - Old Size)
5. **Growth %**: Percentage growth relative to the old size
6. **Objects**: Change in object count (if available)

### Common Leak Patterns

- **High growth percentage with high object count increase**: Likely a collection leak (e.g., continuously adding to a slice/map without removing)
- **High growth percentage with stable object count**: Possibly growing individual objects (e.g., expanding buffers)
- **Steady growth across multiple snapshots**: Confirms a true leak rather than temporary allocation

## Best Practices

1. **Take multiple snapshots** over time to confirm consistent growth patterns
2. **Use a lower threshold** (e.g., 5%) for initial investigation, then increase it to focus on the most significant leaks
3. **Look for both absolute and percentage growth** - small percentage growth of a large allocation can still be significant
4. **Compare with application behavior** - correlate memory growth with specific operations or user actions
5. **Use with other profiling tools** - combine with CPU profiles to see what code is manipulating the leaking objects

## Troubleshooting

- **No leaks detected**: Try lowering the threshold or running the application longer between snapshots
- **Too many results**: Increase the threshold to focus on the most significant leaks
- **Unknown types**: Some allocations may show as "unknown" if type information is not available in the profile

## Advanced Usage

For more advanced memory leak investigations, consider:

1. Using multiple snapshots over time to track growth patterns
2. Correlating memory growth with specific application events or operations
3. Using custom instrumentation to tag allocations with additional context
4. Combining with CPU profiles to identify code that manipulates the leaking objects
