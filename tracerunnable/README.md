# Trace Runnable Goroutines Tool

A Go tool that analyzes execution traces and lists goroutines that are runnable at a specific time offset.

## Usage

```bash
go run main.go -trace <tracefile> -time <offset>
```

### Examples

```bash
# List runnable goroutines at 1.5 seconds into the trace
go run main.go -trace trace.out -time 1.5s

# List runnable goroutines at 100 milliseconds into the trace  
go run main.go -trace trace.out -time 100ms

# List runnable goroutines at 2500 microseconds into the trace
go run main.go -trace trace.out -time 2500us
```

## How to Generate a Trace File

To generate a trace file for analysis:

```go
package main

import (
    "os"
    "runtime/trace"
)

func main() {
    f, _ := os.Create("trace.out")
    defer f.Close()
    
    trace.Start(f)
    defer trace.Stop()
    
    // Your program logic here
}
```

## Output

The tool outputs a list of goroutine IDs that are in the runnable state at the specified time offset. A goroutine is considered runnable if it's ready to run but not currently running (waiting for a processor).