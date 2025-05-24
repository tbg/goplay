package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
	"trace"
)

var (
	traceFile = flag.String("trace", "", "path to trace file")
	timeOffset = flag.String("time", "", "time offset in the trace (e.g., 1.5s, 100ms, 2500us)")
)

func main() {
	flag.Parse()
	
	if *traceFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -trace <tracefile> -time <offset>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -trace trace.out -time 1.5s\n", os.Args[0])
		os.Exit(1)
	}
	
	if *timeOffset == "" {
		fmt.Fprintf(os.Stderr, "Error: time offset is required\n")
		os.Exit(1)
	}
	
	duration, err := time.ParseDuration(*timeOffset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing time offset: %v\n", err)
		os.Exit(1)
	}
	
	if err := analyzeTrace(*traceFile, duration); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func analyzeTrace(filename string, targetTime time.Duration) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open trace file: %w", err)
	}
	defer f.Close()
	
	reader, err := trace.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create trace reader: %w", err)
	}
	
	// Track goroutine states at the target time
	runnableGoroutines := make(map[uint64]bool)
	targetTimeNs := int64(targetTime.Nanoseconds())
	
	// Find the closest event to our target time and track goroutine states
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to read trace event: %w", err)
		}
		
		eventTimeNs := int64(event.Time())
		
		// If we've passed our target time, stop processing
		if eventTimeNs > targetTimeNs {
			break
		}
		
		// Track goroutine state changes
		if gid := event.Goroutine(); gid != trace.NoGoroutine {
			switch event.Kind() {
			case trace.EventStateTransition:
				// Handle goroutine state transitions
				if st := event.StateTransition(); st.Goroutine != trace.NoGoroutine {
					// Check if transitioning to runnable state
					if st.New == trace.GoRunnable {
						runnableGoroutines[uint64(st.Goroutine)] = true
					} else if st.New == trace.GoRunning || 
							 st.New == trace.GoWaiting ||
							 st.New == trace.GoSyscall ||
							 st.New == trace.GoNotExist {
						// Remove from runnable if transitioning to other states
						delete(runnableGoroutines, uint64(st.Goroutine))
					}
				}
			}
		}
	}
	
	// Output the results
	fmt.Printf("Runnable goroutines at time offset %v:\n", targetTime)
	if len(runnableGoroutines) == 0 {
		fmt.Println("  (none)")
	} else {
		for gid := range runnableGoroutines {
			fmt.Printf("  Goroutine %d\n", gid)
		}
	}
	
	return nil
}