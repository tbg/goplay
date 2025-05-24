// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/pprof"
)

type Worker interface {
	Work(i int) int
}

type WorkerA struct{}

func (w *WorkerA) Work(i int) int {
	return i + 1
}

type WorkerB struct{}

func (w *WorkerB) Work(i int) int {
	return i + 2
}

type WorkerC struct{}

func (w *WorkerC) Work(i int) int {
	return i + 3
}

const (
	profile = false
	iters   = 20_000
)

func main() {
	workers := make([]Worker, 100_000)
	for i := range workers {
		switch rand.Intn(50) {
		case 0:
			workers[i] = &WorkerA{}
		case 1:
			workers[i] = &WorkerB{}
		default:
			workers[i] = &WorkerC{}
		}
	}

	if profile {
		f, _ := os.Create("cpu.prof")
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	sum := 0
	for i := 0; i < iters; i++ {
		for j, w := range workers {
			sum += w.Work(j)
		}
	}
	fmt.Println(sum)
}
