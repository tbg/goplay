package main

import "fmt"

type things struct{
	a,b,c int
	d [1024]byte
}

func main() {
	t1 := things{}
	t2 := t1
	fmt.Println(t2)
}