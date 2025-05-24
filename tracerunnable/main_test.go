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

	// Generate many more goroutines doing intensive work
	var wg sync.WaitGroup
	startTime := time.Now()
	
	// Create significantly more goroutines than GOMAXPROCS to ensure contention
	// This greatly increases the chance of having runnable goroutines and trace events
	numGoroutines := 200 // Much more than typical GOMAXPROCS (usually 2-16)
	workDuration := 100 * time.Millisecond // Longer work duration to generate more events
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Do work that creates many scheduling events
			endTime := startTime.Add(workDuration)
			iteration := 0
			for time.Now().Before(endTime) {
				// Computational work
				sum := 0
				for j := 0; j < 5000; j++ {
					sum += j * j
				}
				
				// Frequent yielding to create more scheduling events and contention
				iteration++
				if iteration%5 == 0 {
					time.Sleep(time.Microsecond * 10) // Very frequent, brief sleeps
				}
				
				// Channel operations to create more trace events
				if iteration%10 == 0 {
					ch := make(chan bool, 1)
					ch <- true
					<-ch
				}
				
				_ = sum % 1000
			}
		}(i)
	}
	
	// Create many background goroutines with different blocking patterns
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Various blocking operations to create scheduling events
			if id%3 == 0 {
				time.Sleep(time.Millisecond * 10) // Short sleep
			} else if id%3 == 1 {
				time.Sleep(time.Millisecond * 50) // Medium sleep  
			} else {
				// Channel operations
				ch := make(chan int, 1)
				go func() {
					time.Sleep(time.Millisecond * 20)
					ch <- 1
				}()
				<-ch
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Stop tracing
	trace.Stop()
	traceFile.Close()

	// Test the analyzeTrace function at different offsets
	t.Run("0ms offset", func(t *testing.T) {
		t.Logf("Testing trace analysis at 0ms offset")
		err := analyzeTrace(traceFile.Name(), 0*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 0ms offset: %v", err)
		}
	})

	t.Run("5ms offset", func(t *testing.T) {
		t.Logf("Testing trace analysis at 5ms offset")
		err := analyzeTrace(traceFile.Name(), 5*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 5ms offset: %v", err)
		}
	})
	
	t.Run("25ms offset", func(t *testing.T) {
		t.Logf("Testing trace analysis at 25ms offset")
		err := analyzeTrace(traceFile.Name(), 25*time.Millisecond)
		if err != nil {
			t.Errorf("analyzeTrace failed at 25ms offset: %v", err)
		}
	})
}