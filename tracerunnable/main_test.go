package main

import (
	"os"
	"runtime/trace"
	"sync"
	"testing"
	"time"
)

func TestAnalyzeTrace(t *testing.T) {
	// Create a temporary trace file
	traceFile, err := os.CreateTemp("", "test_trace_*.out")
	if err != nil {
		t.Fatalf("Failed to create temp trace file: %v", err)
	}
	defer os.Remove(traceFile.Name())
	defer traceFile.Close()

	// Start tracing
	if err := trace.Start(traceFile); err != nil {
		t.Fatalf("Failed to start tracing: %v", err)
	}

	// Generate goroutines doing busywork for 10ms
	var wg sync.WaitGroup
	startTime := time.Now()
	
	// Create many more goroutines than GOMAXPROCS to ensure contention
	// This increases the chance of having runnable goroutines
	numGoroutines := 50 // Much more than typical GOMAXPROCS
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Do busywork for approximately 10ms
			for time.Since(startTime) < 10*time.Millisecond {
				// Simulate intensive work by doing more computation
				sum := 0
				for j := 0; j < 10000; j++ {
					sum += j * j
				}
				
				// Yield occasionally to create more scheduling events
				if sum%1000 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Stop tracing
	trace.Stop()
	traceFile.Close()

	// Test the analyzeTrace function at 0ms offset
	t.Run("0ms offset", func(t *testing.T) {
		err := analyzeTrace(traceFile.Name(), 0*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 0ms offset: %v", err)
		}
	})

	// Test the analyzeTrace function at 5ms offset
	t.Run("5ms offset", func(t *testing.T) {
		err := analyzeTrace(traceFile.Name(), 5*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 5ms offset: %v", err)
		}
	})

	// Test the analyzeTrace function at 25ms offset
	t.Run("25ms offset", func(t *testing.T) {
		err := analyzeTrace(traceFile.Name(), 25*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 25ms offset: %v", err)
		}
	})
}