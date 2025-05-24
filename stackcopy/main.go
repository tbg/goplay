package main

import (
	"fmt"
	"math/rand"
)

type things struct {
	a, b, c int
	d       [1024]byte
}

func main() {
	t1 := things{}
	t1.a = rand.Intn(100)
	t1.b = rand.Intn(100)
	t1.c = rand.Intn(100)
	for i := range t1.d {
		t1.d[i] = byte(rand.Intn(256))
	}
	t2 := t1
	fmt.Println(t2)
}
