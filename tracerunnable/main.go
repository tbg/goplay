package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	exptrace "golang.org/x/exp/trace"
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
	
	// Parse the trace directly from the file reader
	reader, err := exptrace.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create trace reader: %w", err)
	}
	
	// Track goroutine states at the target time
	runnableGoroutines := make(map[exptrace.GoID]bool)
	targetTimeNs := targetTime.Nanoseconds()
	var traceStartTime exptrace.Time
	traceStartTimeSet := false
	
	// Process all events up to the target time
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read trace event: %w", err)
		}
		
		// Set the trace start time from the first event
		if !traceStartTimeSet {
			traceStartTime = event.Time()
			traceStartTimeSet = true
		}
		
		// Calculate time offset from start of trace
		eventTimeOffset := event.Time() - traceStartTime
		
		// If we've passed our target time, stop processing
		if int64(eventTimeOffset) > int64(targetTimeNs) {
			break
		}
		
		// Track goroutine state changes
		switch event.Kind() {
		case exptrace.EventStateTransition:
			stateTransition := event.StateTransition()
			if stateTransition.Resource.Kind == exptrace.ResourceGoroutine {
				gid := stateTransition.Resource.Goroutine()
				
				// Get the goroutine's state transition (from, to)
				_, toState := stateTransition.Goroutine()
				
				// Check if transitioning to runnable state
				if toState == exptrace.GoRunnable {
					runnableGoroutines[gid] = true
				} else if toState == exptrace.GoRunning || 
						 toState == exptrace.GoWaiting ||
						 toState == exptrace.GoSyscall ||
						 toState == exptrace.GoNotExist {
					// Remove from runnable if transitioning to other states
					delete(runnableGoroutines, gid)
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