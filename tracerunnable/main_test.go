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
	
	// Create multiple goroutines to do busywork
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Do busywork for approximately 10ms
			for time.Since(startTime) < 10*time.Millisecond {
				// Simulate work by doing some computation
				sum := 0
				for j := 0; j < 1000; j++ {
					sum += j
				}
				
				// Yield occasionally to create more scheduling events
				if sum%100 == 0 {
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
}