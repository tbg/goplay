package main

import (
	"context"
)

func main() {
	ctx := context.Background()
	var x int
	for j := 0; j < 10000000; j++ {
		ctx := context.WithValue(ctx, "x", x)
		if x%2 == 0 {
			x += doWork(ctx)
		}
		if x%3 == 0 {
			x += doWork(ctx)
		}
	}
}

func doWork(ctx context.Context) int {
	x := ctx.Value("x").(int)
	for i := 0; i < 1000; i++ {
		x = (x + 1) % 42
	}
	return x
}
